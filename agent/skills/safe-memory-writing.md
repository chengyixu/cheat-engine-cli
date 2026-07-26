---
name: safe-memory-writing
description: Preview, confirm, execute, and verify authorized memory writes.
---

First run the intended command with `--dry-run`. Confirm the PID, address, encoded bytes, and target authorization. Execute with `--yes --verify`. Writes fail closed without `--yes`, and diagnostics are emitted as structured JSON on stderr.
