---
name: remote-administration
description: Inspect and deliberately mutate the ceserver filesystem and runtime settings.
---

Use `cecli server path` and `cecli server options list` before changing server state. Name diagnostic connections with `--connection-name` when correlating server logs. Remote reads use `cecli remote ls|stat|get`. Remote writes, directory changes, deletes, permission changes, option changes, module or extension loads, speed changes, allocation, free, protection changes, remote-thread creation, and `server terminate` require a `--dry-run` review followed by explicit `--yes`.
