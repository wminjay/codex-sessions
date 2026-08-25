# Security Policy

## Reporting a vulnerability

Please report security issues privately through GitHub's security advisory feature instead of opening a public issue.

## Data handling

Codex Sessions is a local tool. It reads Codex session metadata and starts the installed Codex CLI. It does not send session contents to its own service, store API keys, or modify Codex session files.

The `--yolo` option is an explicit passthrough to Codex and is disabled by default.
