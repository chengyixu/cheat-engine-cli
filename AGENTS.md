# Cheat Engine CLI Development

- Keep the upstream Cheat Engine source intact unless a CLI feature requires a focused compatibility patch.
- Put the Go CLI in `cmd/cecli` and reusable packages in `internal/`.
- Preserve JSON output schemas within a minor release and emit diagnostics only on stderr.
- Treat memory writes, process termination, injection, and debugger controls as destructive operations requiring explicit confirmation.
- Add protocol tests with a fake `ceserver`; do not require a privileged live process for unit tests.
- Run `go test ./...`, `go vet ./...`, and the CLI smoke checks before release.
