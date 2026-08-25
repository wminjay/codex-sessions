# Codex Sessions

[![CI](https://github.com/wminjay/codex-sessions/actions/workflows/ci.yml/badge.svg)](https://github.com/wminjay/codex-sessions/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/wminjay/codex-sessions)](https://github.com/wminjay/codex-sessions/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A fast, polished terminal session explorer for Codex CLI.

Codex Sessions brings local sessions created with account login, custom API providers, VS Code, and different project directories into one searchable view. Pick a session, choose a provider, and continue through Codex's own `resume` or `fork` command.

> Unofficial community tool. It is not affiliated with or endorsed by OpenAI.

[简体中文](README.zh-CN.md)

```text
◆ CODEX SESSIONS  /  Directories              67 projects · 141 sessions
  /  Search projects, titles, providers, models, or IDs
────────────────────────────────────────────────────────────────────────
› codex-sessions      17 sessions   openai:17             just now
  demo-api             7 sessions   custom:2  openai:5     15h ago
  terminal-notes       3 sessions   custom:1  openai:2      1d ago
  HushWire             4 sessions   custom:3  openai:1      1d ago
────────────────────────────────────────────────────────────────────────
  ↑↓ navigate    enter open    / search    q quit
```

## Why

Codex's built-in resume picker is intentionally contextual. That is convenient most of the time, but it can make a known session harder to find after changing project directories or model providers. Codex Sessions provides a read-only, global view without replacing Codex's session engine.

It always hands the selected UUID back to the official CLI:

```text
codex resume SESSION_ID
codex fork SESSION_ID
```

See the [official Codex CLI command reference](https://learn.chatgpt.com/docs/developer-commands?surface=cli) for the underlying commands.

## Highlights

- One view across `openai`, `custom`, VS Code, and multiple Codex homes
- Directory-first navigation, followed by sessions sorted by recency
- Search across project paths, renamed titles, first prompts, providers, models, and UUIDs
- Resume with the original provider, account login, or a custom API provider
- Fork without changing the original conversation
- Simple `-yolo` switch with a prominent UI warning
- Read-only SQLite access with JSONL rollout fallback
- Empty metadata-only sessions are hidden
- Pure Go, no Python, CGO, `sqlite3`, or `fzf` dependency
- Single binaries for macOS, Linux, and Windows on x86-64 and ARM64

## Install

### Go

Requires Go 1.24.2 or newer:

```bash
go install github.com/wminjay/codex-sessions/cmd/codex-sessions@latest
```

### Release binary

Download the archive for your platform from [GitHub Releases](https://github.com/wminjay/codex-sessions/releases), then place `codex-sessions` (or `codex-sessions.exe`) on your `PATH`.

Optional short alias:

```bash
alias cxs='codex-sessions'
```

## Usage

```bash
codex-sessions
```

The first screen lists directories. Press `enter` to open one and see its sessions.

### Keys

| Key | Directory view | Session view |
|---|---|---|
| `↑` / `↓`, `j` / `k` | Navigate | Navigate |
| `enter` | Open directory | Resume with original provider |
| `/` | Search directories and their sessions | Search sessions |
| `c` | — | Resume with `custom` provider |
| `a` | — | Resume with account (`openai`) provider |
| `f` | — | Fork with original provider |
| `C` / `A` | — | Fork with custom/account provider |
| `esc`, `b`, `←` | Quit | Return to directories |
| `q` | Quit | Quit |

### Common commands

```bash
# Show the UI in YOLO mode
codex-sessions -yolo

# Inspect without launching
codex-sessions --list --limit 20
codex-sessions --json --query demo-api

# Resume a known UUID with a custom provider
codex-sessions --session 01a02e1c --provider custom

# Verify the exact generated command
codex-sessions --session 01a02e1c --provider custom -yolo --dry-run

# Merge independent stores
codex-sessions \
  --codex-home ~/.codex \
  --codex-home ~/.codex-work
```

Run `codex-sessions --help` for every option.

## Custom providers

If the `custom` provider already exists in the selected store's base `config.toml`, no extra option is needed. If it lives in a named Codex profile, pass it once when opening the explorer:

```bash
codex-sessions --custom-profile custom
```

A custom launch then uses both the profile and provider override:

```text
codex resume SESSION_ID -p custom -c 'model_provider="custom"'
```

Codex Sessions never reads or stores API keys.

## Safety and storage

The tool discovers `$CODEX_HOME` (default `~/.codex`) and `$CODEX_SQLITE_HOME`, then opens the newest `state_*.sqlite` in read-only mode. It detects available columns before querying because Codex's local database is an internal implementation detail, not a public stable API. If SQLite is unavailable, it scans `sessions/**/rollout-*.jsonl`.

Title selection follows this order:

1. Renamed session name
2. First user message
3. Preview/title metadata

Records with no readable conversation content are hidden. The tool never updates SQLite or rollout files.

`-yolo` is disabled by default. Internally, Codex Sessions translates it to the alias understood by the installed Codex CLI; it never expands it into a different long-form option. YOLO skips Codex's normal safety workflow, so only use it in an environment where that behavior is intentional.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/codex-sessions
```

Architecture notes are in [docs/architecture.md](docs/architecture.md).

## License

[MIT](LICENSE)
