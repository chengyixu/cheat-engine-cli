# Changelog

All notable changes to Cheat Engine CLI are documented here.

## [0.1.0] - 2026-07-26

### Added

- JSON-first `cecli` command router with human output mode, structured errors, stable exit codes, `--brief`, `--version`, `--dry-run`, `--quiet`, dotted `--fields`, and catalog-derived nested shell completions.
- Command-specific machine-readable help with typed parameters, defaults, enums, conditional constraints, and response-envelope schemas.
- Portable `ceserver` client for version/ABI discovery, processes, modules, threads, memory regions, reads, writes, and architecture inspection.
- Chunked exact-value and byte-pattern scanner with wildcard and alignment support.
- Confirmation-gated memory writes with optional read-back verification.
- Guarded memory allocation, free, protection changes, module loading, and speed control.
- Thread suspend/resume counts for active debug sessions and confirmation-gated remote-thread creation.
- Bounded debug-event tracing with event limits, per-event timeouts, automatic signal continuation, and connection-cleanup detach.
- Hardware breakpoint set/remove commands plus raw thread-context export and verified replacement.
- Ceserver extension loading and explicit-`dlopen` module loading.
- `ceserver` path/runtime option inspection plus verified option changes.
- Per-connection diagnostic naming and confirmation-gated remote server termination.
- Explicit `memory aobscan` support for the upstream server-side scan packet while retaining the portable client scanner as the default.
- Remote filesystem listing, stat, download, upload, directory, delete, and permission commands.
- Named-pipe operations and compressed ELF symbol listing.
- Embedded agent rules and skills plus a local structured issue workflow.
- Cross-platform tests, fake `ceserver` protocol coverage, GoReleaser configuration, and GitHub Actions workflows.
- Search and AI-discovery assets including a GitHub Pages site, JSON-LD SoftwareApplication/HowTo/FAQ/Breadcrumb schemas, `llms.txt`, sitemap, robots policy, social cards, and product marketing context.
- Native local process discovery, memory regions, reads, exact scans, and guarded writes on Windows and macOS through `cecli --native`.
- A macOS debugger-entitlement signing helper and native-specific permission diagnostics.
- An optional standalone `cebridge` transport for accessing the native subset across an explicitly configured machine or VM boundary.
- Native-capable arm64 and x86_64 Darwin release archives built on macOS instead of CGO-disabled cross-compiled stubs.

### Fixed

- Pass the requested parameter to `ceserver` remote-thread creation instead of repeating the start address.
- Initialize memory-protection responses on query failure.
- Close remote file descriptors and handle partial writes in `ceserver` file transfer commands.
- Detach debugger sessions when a graceful close command is received; debug-aware clients also drop the TCP connection directly for compatibility with unpatched servers.
- Replace the unsafe experimental native AOB loop with bounded pattern validation, correct chunk overlap, stable alignment, safe allocation handling, and bounded result framing.
