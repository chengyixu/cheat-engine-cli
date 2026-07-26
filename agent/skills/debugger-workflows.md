---
name: debugger-workflows
description: Collect bounded debug events and safely manage breakpoints or raw thread contexts.
---

Use `cecli debug trace --pid <pid> --events <count> --event-timeout <duration> --dry-run` before attaching. Execute only with `--yes`; the default `--continue auto` ignores virtual create events, `SIGTRAP`, and `SIGSTOP`, while delivering other signals. Breakpoint and thread suspend/resume commands require an already-active `ceserver` debug session. `debug context get` returns a complete base64 blob and SHA-256 digest; `debug context set` validates the embedded size, attaches temporarily, requires `--yes`, and supports `--verify`.
