package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/wminjay/codex-sessions/internal/launcher"
	"github.com/wminjay/codex-sessions/internal/session"
	"github.com/wminjay/codex-sessions/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type options struct {
	homes           stringList
	includeArchived bool
	allSources      bool
	query           string
	list            bool
	json            bool
	limit           int
	sessionID       string
	action          string
	provider        string
	customProfile   string
	codexBin        string
	dryRun          bool
	yolo            bool
	inline          bool
	showVersion     bool
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(arguments []string, stdout, stderr io.Writer) int {
	options, err := parseOptions(arguments, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "codex-sessions:", err)
		return 2
	}
	if options.showVersion {
		fmt.Fprintf(stdout, "codex-sessions %s (%s, %s)\n", version, commit, date)
		return 0
	}
	stores, err := session.DiscoverStores(options.homes)
	if err != nil {
		fmt.Fprintln(stderr, "codex-sessions:", err)
		return 1
	}
	items, err := session.Load(stores, session.LoadOptions{
		IncludeArchived: options.includeArchived,
		AllSources:      options.allSources,
	})
	if err != nil {
		fmt.Fprintln(stderr, "codex-sessions:", err)
		return 1
	}
	if len(items) == 0 {
		fmt.Fprintln(stderr, "codex-sessions: no resumable sessions found")
		return 1
	}
	if options.list || options.json {
		printSessions(stdout, items, options)
		return 0
	}

	var request *launcher.Request
	if options.sessionID != "" {
		selected, err := session.Resolve(items, options.sessionID)
		if err != nil {
			fmt.Fprintln(stderr, "codex-sessions:", err)
			return 2
		}
		request = &launcher.Request{
			Session: selected, Action: options.action, Provider: options.provider,
		}
	} else {
		if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
			fmt.Fprintln(stderr, "codex-sessions: interactive mode requires a terminal; use --list or --session")
			return 2
		}
		request, err = ui.Run(items, ui.Options{
			InitialQuery: options.query, Yolo: options.yolo, Inline: options.inline,
		})
		if err != nil {
			fmt.Fprintln(stderr, "codex-sessions:", err)
			return 1
		}
		if request == nil {
			return 0
		}
	}

	command, err := launcher.Build(*request, launcher.Options{
		CodexBin: options.codexBin, CustomProfile: options.customProfile, Yolo: options.yolo,
	})
	if err != nil {
		fmt.Fprintln(stderr, "codex-sessions:", err)
		return 2
	}
	printable := launcher.Printable(command)
	if options.dryRun {
		fmt.Fprintln(stdout, printable)
		return 0
	}
	fmt.Fprintln(stdout, "Launching:", printable)
	if err := launcher.Run(command); err != nil {
		fmt.Fprintln(stderr, "codex-sessions:", err)
		return launcher.ExitCode(err)
	}
	return 0
}

func parseOptions(arguments []string, stderr io.Writer) (options, error) {
	var result options
	flags := flag.NewFlagSet("codex-sessions", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Var(&result.homes, "codex-home", "Codex home to scan; repeat to merge multiple stores")
	flags.BoolVar(&result.includeArchived, "include-archived", false, "include archived sessions")
	flags.BoolVar(&result.allSources, "all-sources", false, "include exec and subagent sessions")
	flags.StringVar(&result.query, "query", "", "initial search text")
	flags.BoolVar(&result.list, "list", false, "print sessions without opening the TUI")
	flags.BoolVar(&result.json, "json", false, "print list output as JSON")
	flags.IntVar(&result.limit, "limit", 50, "maximum rows printed by --list")
	flags.StringVar(&result.sessionID, "session", "", "launch an exact session ID or unambiguous prefix")
	flags.StringVar(&result.action, "action", launcher.ActionResume, "direct action: resume or fork")
	flags.StringVar(&result.provider, "provider", launcher.ProviderOriginal, "provider: original, openai, or custom")
	flags.StringVar(&result.customProfile, "custom-profile", "", "Codex profile layered for custom launches")
	flags.StringVar(&result.codexBin, "codex-bin", "codex", "Codex executable name or path")
	flags.BoolVar(&result.dryRun, "dry-run", false, "print the launch command without executing it")
	flags.BoolVar(&result.yolo, "yolo", false, "start Codex in YOLO mode")
	flags.BoolVar(&result.inline, "inline", false, "run without the terminal alternate screen")
	flags.BoolVar(&result.showVersion, "version", false, "print build version")
	flags.Usage = func() {
		fmt.Fprintln(stderr, `Codex Sessions — browse every local Codex session across providers.

Usage:
  codex-sessions [options]

Examples:
  codex-sessions
  codex-sessions -yolo
  codex-sessions --list --query demo-api
  codex-sessions --session 01a02e1c --provider custom --dry-run

Options:`)
		flags.PrintDefaults()
	}
	if err := flags.Parse(arguments); err != nil {
		return result, err
	}
	if flags.NArg() != 0 {
		return result, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if result.action != launcher.ActionResume && result.action != launcher.ActionFork {
		return result, fmt.Errorf("--action must be resume or fork")
	}
	switch result.provider {
	case launcher.ProviderOriginal, launcher.ProviderOpenAI, launcher.ProviderCustom:
	default:
		return result, fmt.Errorf("--provider must be original, openai, or custom")
	}
	if result.limit < 0 {
		return result, fmt.Errorf("--limit cannot be negative")
	}
	return result, nil
}

func printSessions(writer io.Writer, items []session.Session, options options) {
	filtered := session.Filter(items, options.query)
	if options.limit < len(filtered) {
		filtered = filtered[:options.limit]
	}
	if options.json {
		type record struct {
			ID        string `json:"id"`
			UpdatedAt string `json:"updated_at"`
			Provider  string `json:"provider"`
			Source    string `json:"source"`
			Model     string `json:"model,omitempty"`
			CWD       string `json:"cwd"`
			Title     string `json:"title"`
			Archived  bool   `json:"archived"`
			CodexHome string `json:"codex_home"`
		}
		records := make([]record, 0, len(filtered))
		for _, item := range filtered {
			records = append(records, record{
				ID: item.ID, UpdatedAt: time.UnixMilli(item.UpdatedMS).Local().Format(time.RFC3339),
				Provider: item.Provider, Source: item.Source, Model: item.Model, CWD: item.CWD,
				Title: item.Title, Archived: item.Archived, CodexHome: item.CodexHome,
			})
		}
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(records)
		return
	}
	fmt.Fprintln(writer, tableRow("UPDATED", "PROVIDER", "SOURCE", "PROJECT", "TITLE"))
	for _, item := range filtered {
		updated := time.UnixMilli(item.UpdatedMS).Local().Format("2006-01-02 15:04")
		fmt.Fprintln(writer, tableRow(updated, item.Provider, item.Source, item.Project(), item.Title))
	}
	fmt.Fprintf(writer, "\n%d/%d sessions\n", len(filtered), len(session.Filter(items, options.query)))
}

func tableRow(updated, provider, source, project, title string) string {
	return pad(updated, 17) + " " + pad(provider, 10) + " " + pad(source, 9) + " " + pad(project, 22) + " " + title
}

func pad(value string, width int) string {
	if runewidth.StringWidth(value) > width {
		value = runewidth.Truncate(value, width, "…")
	}
	return runewidth.FillRight(value, width)
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
