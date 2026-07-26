---
name: workflow
description: Safe execution sequence for process and memory operations.
---

1. Run `cecli server info` and confirm the endpoint identity.
2. Discover the target with `cecli process list`; never guess a PID.
3. Inspect architecture, modules, and regions before reading memory.
4. Use the smallest practical read or scan range.
5. Preview writes with `--dry-run`, then require `--yes` and use `--verify`.
6. For debugger work, bound `debug trace` with `--events` and `--event-timeout`; use the default `--continue auto` unless signal behavior is explicitly understood.
7. Treat raw context blobs as architecture-specific binary state; preserve the size header and prefer `--verify` when setting one.
8. Record unexpected behavior with `cecli issue create`.
