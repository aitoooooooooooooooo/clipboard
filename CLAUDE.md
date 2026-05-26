APM_RULES {

## Error Handling

- Use Go idiomatic error returns (`return err` or `return fmt.Errorf("context: %w", err)`).
- Never use `panic` or `log.Fatal` in library code — only in `main()` for unrecoverable startup failures.
- Always wrap errors with context (`fmt.Errorf("failed to open database: %w", err)`).
- Log errors at the point they are handled, not where they originate.

## Logging

- Use `log/slog` (Go standard library, Go 1.21+) for structured logging.
- Log levels: `Error` for failures, `Warn` for degraded state, `Info` for lifecycle events, `Debug` for detailed tracing.
- Never log sensitive data: clipboard content, pairing codes, cryptographic keys, or TLS certificates.

## Cross-Platform Code

- Use Go build tags (`//go:build darwin`) for platform-specific code.
- Platform-specific files: `*_darwin.go` for macOS, `*_windows.go` for Windows.
- Shared logic goes in files without build tags.
- CGO is required for SQLite (`go-sqlite3`) — ensure `CGO_ENABLED=1` in build scripts.
- Test on both platforms before considering a task complete.

## Code Organization

- `cmd/clipboardsync/` — application entry point only.
- `internal/` — private packages, not importable externally.
- `pkg/` — shared interfaces and models (if needed across packages).
- Each package has a single clear responsibility.
- Keep package APIs small — expose only what is needed.

## Testing

- Write unit tests for storage operations, pairing logic, and message serialization.
- Use table-driven tests (`[]struct{...}`) for multiple input scenarios.
- Integration tests for network discovery and sync protocol use localhost with two instances.
- Run `go test ./...` before marking any task complete.

## Security

- All network communication must use TLS 1.3 — no plaintext sockets.
- Pairing codes expire after 60 seconds — enforce this strictly.
- Store cryptographic material (private keys, peer public keys) in the SQLite database, not in config files.
- Validate message sizes before processing — reject messages exceeding 50MB + overhead.

## Dependencies

- Prefer Go standard library when possible (`net`, `crypto/tls`, `log/slog`).
- Minimize external dependencies — each dependency must be well-maintained and widely used.
- Pin dependency versions in `go.mod`.

## Version Control

- 基础分支: `master`
- 分支命名: `type/short-description`（如 `feat/project-setup`、`fix/eviction-bug`）
- 提交信息: `type: description`，类型包括 `feat`, `fix`, `refactor`, `docs`, `test`, `chore`

} //APM_RULES
