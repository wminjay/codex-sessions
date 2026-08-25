# Contributing

Issues and focused pull requests are welcome.

## Development setup

1. Install Go 1.27.0 or newer.
2. Clone the repository.
3. Run `go test ./...` and `go vet ./...`.
4. Build with `go build ./cmd/codex-sessions`.

Keep session storage access read-only. New schema compatibility code should detect columns or fields before using them, and tests must use temporary fixture data rather than a developer's real Codex home.
