# Architecture

Codex Sessions has three runtime layers:

```text
Codex local state
  ├─ state_*.sqlite (preferred, opened read-only)
  └─ sessions/**/rollout-*.jsonl (fallback)
                │
                ▼
        session discovery
        filtering + grouping
                │
                ▼
        Bubble Tea terminal UI
                │ selected UUID/provider/action
                ▼
        installed Codex CLI
        resume/fork SESSION_ID
```

## Packages

- `internal/session`: store discovery, schema-tolerant SQLite queries, rollout fallback, filtering, and directory grouping.
- `internal/ui`: the directory-first Bubble Tea interface.
- `internal/launcher`: construction and execution of Codex `resume` / `fork` commands.
- `cmd/codex-sessions`: flags, non-interactive output, version metadata, and process orchestration.

## Compatibility strategy

The local SQLite schema is internal to Codex. The loader finds the highest `state_*.sqlite` version, checks the `threads` columns at runtime, and builds its query only from columns that exist. It selects a label in this order: `name`, `first_user_message`, `preview`, then `title`.

The database is opened using `mode=ro` and `PRAGMA query_only = ON`. The launcher never updates metadata to change providers; it passes a provider override to the official Codex CLI for that invocation.
