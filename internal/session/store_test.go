package session

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLoadSQLiteMergesProvidersAndHidesEmptySessions(t *testing.T) {
	root := t.TempDir()
	database, err := sql.Open("sqlite", filepath.Join(root, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`
		CREATE TABLE threads (
			id TEXT PRIMARY KEY,
			updated_at INTEGER NOT NULL,
			source TEXT NOT NULL,
			model_provider TEXT NOT NULL,
			cwd TEXT NOT NULL,
			title TEXT NOT NULL,
			name TEXT,
			preview TEXT,
			first_user_message TEXT,
			archived INTEGER NOT NULL DEFAULT 0,
			model TEXT
		);
		INSERT INTO threads VALUES
			('openai-1', 30, 'cli', 'openai', '/work/one', 'Generated title', '', 'First account message', 'First account message', 0, 'gpt-a'),
			('custom-1', 20, 'vscode', 'custom', '/work/two', '', 'Renamed custom', 'First custom message', 'First custom message', 0, 'gpt-b'),
			('empty-1', 15, 'cli', 'openai', '/work/two', '', '', '', '', 0, 'gpt-b'),
			('exec-1', 40, 'exec', 'openai', '/work/three', 'Automation', '', 'Automation', 'Automation', 0, 'gpt-c'),
			('archived-1', 10, 'cli', 'openai', '/work/four', 'Archived', '', 'Archived', 'Archived', 1, 'gpt-d');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	store := Store{CodexHome: root, SQLiteHome: root}
	items, err := Load([]Store{store}, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 interactive sessions, got %d", len(items))
	}
	if items[0].ID != "openai-1" || items[0].Title != "First account message" {
		t.Fatalf("unexpected first session: %#v", items[0])
	}
	if items[1].Title != "Renamed custom" {
		t.Fatalf("rename should win, got %q", items[1].Title)
	}

	all, err := Load([]Store{store}, LoadOptions{IncludeArchived: true, AllSources: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 non-empty sessions, got %d", len(all))
	}
}

func TestSQLiteStoreIsOpenedReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state_1.sqlite")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("CREATE TABLE threads (id TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	readOnly, err := sql.Open("sqlite", readOnlySQLiteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	if _, err := readOnly.Exec("CREATE TABLE forbidden (id TEXT)"); err == nil {
		t.Fatal("expected write through read-only SQLite DSN to fail")
	}
}

func TestRolloutFallbackUsesFirstRealUserMessage(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "sessions", "2026", "08", "25")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	events := []any{
		map[string]any{"type": "session_meta", "payload": map[string]any{
			"id": "rollout-1", "cwd": "/work/fallback", "model_provider": "custom",
		}},
		map[string]any{"type": "response_item", "payload": map[string]any{
			"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "<environment_context>ignored</environment_context>"},
			},
		}},
		map[string]any{"type": "response_item", "payload": map[string]any{
			"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "第一句话"},
			},
		}},
	}
	file, err := os.Create(filepath.Join(directory, "rollout-test.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	items, err := Load([]Store{{CodexHome: root, SQLiteHome: root}}, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "第一句话" || items[0].Provider != "custom" {
		t.Fatalf("unexpected rollout session: %#v", items)
	}
}

func TestGroupingFilteringAndResolution(t *testing.T) {
	items := []Session{
		{ID: "aaa-111", Title: "Account task", Provider: "openai", CWD: "/work/app", UpdatedMS: 20},
		{ID: "bbb-222", Title: "Custom task", Provider: "custom", CWD: "/work/app", UpdatedMS: 10},
		{ID: "ccc-333", Title: "Other", Provider: "openai", CWD: "/work/tool", UpdatedMS: 30},
	}
	directories := Group(items)
	if len(directories) != 2 || directories[1].ProviderSummary() != "custom:1  openai:1" {
		t.Fatalf("unexpected groups: %#v", directories)
	}
	filtered := FilterDirectories(directories, "custom task")
	if len(filtered) != 1 || filtered[0].CWD != "/work/app" {
		t.Fatalf("unexpected directory filter: %#v", filtered)
	}
	resolved, err := Resolve(items, "bbb")
	if err != nil || resolved.ID != "bbb-222" {
		t.Fatalf("unexpected resolution: %#v %v", resolved, err)
	}
}

func TestCleanTextRemovesTerminalControlCharacters(t *testing.T) {
	got := cleanText("hello\x1b[31m\nworld")
	if got != "hello [31m world" {
		t.Fatalf("unexpected sanitized text: %q", got)
	}
}
