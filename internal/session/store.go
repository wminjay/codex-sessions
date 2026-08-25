package session

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	_ "modernc.org/sqlite"
)

type LoadOptions struct {
	IncludeArchived bool
	AllSources      bool
}

func DiscoverStores(explicitHomes []string) ([]Store, error) {
	var stores []Store
	if len(explicitHomes) > 0 {
		for _, home := range explicitHomes {
			expanded, err := expandPath(home)
			if err != nil {
				return nil, err
			}
			stores = append(stores, Store{CodexHome: expanded, SQLiteHome: expanded})
		}
	} else {
		codexHome := os.Getenv("CODEX_HOME")
		if codexHome == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("resolve user home: %w", err)
			}
			codexHome = filepath.Join(userHome, ".codex")
		}
		var err error
		codexHome, err = expandPath(codexHome)
		if err != nil {
			return nil, err
		}
		sqliteHome := os.Getenv("CODEX_SQLITE_HOME")
		if sqliteHome == "" {
			sqliteHome = codexHome
		}
		sqliteHome, err = expandPath(sqliteHome)
		if err != nil {
			return nil, err
		}
		stores = []Store{{CodexHome: codexHome, SQLiteHome: sqliteHome}}
	}

	seen := make(map[string]bool)
	deduplicated := make([]Store, 0, len(stores))
	for _, store := range stores {
		key := store.CodexHome + "\x00" + store.SQLiteHome
		if !seen[key] {
			deduplicated = append(deduplicated, store)
			seen[key] = true
		}
	}
	return deduplicated, nil
}

func Load(stores []Store, options LoadOptions) ([]Session, error) {
	merged := make(map[string]Session)
	var loadErrors []error
	for _, store := range stores {
		items, err := loadSQLite(store, options)
		if err != nil {
			loadErrors = append(loadErrors, err)
		}
		if len(items) == 0 {
			items, err = loadRollouts(store, options)
			if err != nil {
				loadErrors = append(loadErrors, err)
			}
		}
		for _, item := range items {
			previous, exists := merged[item.ID]
			if !exists || item.UpdatedMS > previous.UpdatedMS {
				merged[item.ID] = item
			}
		}
	}
	if len(merged) == 0 && len(loadErrors) > 0 {
		return nil, errors.Join(loadErrors...)
	}
	items := make([]Session, 0, len(merged))
	for _, item := range merged {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedMS == items[j].UpdatedMS {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedMS > items[j].UpdatedMS
	})
	return items, nil
}

func loadSQLite(store Store, options LoadOptions) ([]Session, error) {
	databasePath, err := latestStateDatabase(store.SQLiteHome)
	if err != nil || databasePath == "" {
		return nil, err
	}
	dsnURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(databasePath)}
	database, err := sql.Open("sqlite", dsnURL.String()+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", databasePath, err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	if _, err := database.Exec("PRAGMA query_only = ON"); err != nil {
		return nil, fmt.Errorf("set %s read-only: %w", databasePath, err)
	}

	columns, err := threadColumns(database)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", databasePath, err)
	}
	if !columns["id"] {
		return nil, nil
	}
	query := fmt.Sprintf(`
		SELECT
			%s AS session_id,
			%s AS display_title,
			%s AS updated_ms,
			%s AS provider,
			%s AS source,
			%s AS cwd,
			%s AS model,
			%s AS archived
		FROM threads`,
		quoteIdentifier("id"),
		firstNonemptyExpression(columns, []string{"name", "first_user_message", "preview", "title"}, quoteIdentifier("id")),
		timestampExpression(columns),
		columnOrLiteral(columns, "model_provider", "'unknown'"),
		columnOrLiteral(columns, "source", "'unknown'"),
		columnOrLiteral(columns, "cwd", "''"),
		columnOrLiteral(columns, "model", "''"),
		columnOrLiteral(columns, "archived", "0"),
	)
	rows, err := database.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", databasePath, err)
	}
	defer rows.Close()

	items := make([]Session, 0)
	for rows.Next() {
		var item Session
		var archived int64
		if err := rows.Scan(
			&item.ID, &item.Title, &item.UpdatedMS, &item.Provider,
			&item.Source, &item.CWD, &item.Model, &archived,
		); err != nil {
			return nil, fmt.Errorf("scan %s: %w", databasePath, err)
		}
		item.ID = cleanText(item.ID)
		item.Title = cleanText(item.Title)
		item.Provider = fallback(cleanText(item.Provider), "unknown")
		item.Source = fallback(cleanText(item.Source), "unknown")
		item.CWD = cleanText(item.CWD)
		item.Model = cleanText(item.Model)
		item.Archived = archived != 0
		item.CodexHome = store.CodexHome
		item.SQLiteHome = store.SQLiteHome
		if item.Title == "" || item.Title == item.ID {
			continue
		}
		if item.Archived && !options.IncludeArchived {
			continue
		}
		if !options.AllSources && !isInteractiveSource(item.Source) {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", databasePath, err)
	}
	return items, nil
}

func threadColumns(database *sql.DB) (map[string]bool, error) {
	rows, err := database.Query("PRAGMA table_info(threads)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

var stateDatabasePattern = regexp.MustCompile(`^state_(\d+)\.sqlite$`)

func latestStateDatabase(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", root, err)
	}
	type candidate struct {
		path    string
		version int
		modTime int64
	}
	var candidates []candidate
	for _, entry := range entries {
		name := entry.Name()
		version := -1
		if name == "state.sqlite" {
			version = 0
		} else {
			match := stateDatabasePattern.FindStringSubmatch(name)
			if match == nil {
				continue
			}
			version, _ = strconv.Atoi(match[1])
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path: filepath.Join(root, name), version: version, modTime: info.ModTime().UnixNano(),
		})
	}
	if len(candidates) == 0 {
		return "", nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].version == candidates[j].version {
			return candidates[i].modTime > candidates[j].modTime
		}
		return candidates[i].version > candidates[j].version
	})
	return candidates[0].path, nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func firstNonemptyExpression(columns map[string]bool, candidates []string, fallbackExpression string) string {
	parts := make([]string, 0, len(candidates)+1)
	for _, name := range candidates {
		if columns[name] {
			parts = append(parts, "NULLIF("+quoteIdentifier(name)+", '')")
		}
	}
	parts = append(parts, fallbackExpression)
	return "COALESCE(" + strings.Join(parts, ", ") + ")"
}

func timestampExpression(columns map[string]bool) string {
	parts := make([]string, 0, 5)
	for _, name := range []string{"recency_at_ms", "updated_at_ms"} {
		if columns[name] {
			parts = append(parts, "NULLIF("+quoteIdentifier(name)+", 0)")
		}
	}
	if columns["updated_at"] {
		parts = append(parts, quoteIdentifier("updated_at")+" * 1000")
	}
	if columns["created_at"] {
		parts = append(parts, quoteIdentifier("created_at")+" * 1000")
	}
	parts = append(parts, "0")
	return "COALESCE(" + strings.Join(parts, ", ") + ")"
}

func columnOrLiteral(columns map[string]bool, name, literal string) string {
	if columns[name] {
		return quoteIdentifier(name)
	}
	return literal
}

func isInteractiveSource(source string) bool {
	if source == "cli" || source == "vscode" || source == "app" {
		return true
	}
	trimmed := strings.TrimSpace(source)
	return source != "exec" && !strings.HasPrefix(trimmed, "{")
}

func cleanText(value string) string {
	safe := strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value)
	return strings.Join(strings.Fields(safe), " ")
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	return filepath.Clean(absolute), nil
}
