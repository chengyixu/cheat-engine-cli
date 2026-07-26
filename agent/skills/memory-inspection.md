---
name: memory-inspection
description: Read, decode, map, and scan target process memory.
---

Use `cecli memory regions --pid <pid>` to identify valid ranges. Read bytes with `cecli memory read --pid <pid> --address <address> --size <bytes>`. Decode primitives with `--format typed --type u32|i32|f32|u64|f64`. Scan exact bytes with `cecli memory scan --pattern "48 8B ?? FF"` or typed values with `--type i32 --value 100`. Prefer this portable scanner; use confirmation-gated `memory aobscan` only when the upstream server-side packet itself must be tested against the bundled patched server.
