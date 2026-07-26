# Cheat Engine CLI command reference

`cecli` is a stateless client for Cheat Engine `ceserver`. Every process operation opens the target, performs one bounded task, and closes the remote handle. JSON is the default output mode; add `--human` for terminal tables and hex dumps.

## Global options

| Option | Purpose |
|---|---|
| `--endpoint host:port` | Select the `ceserver` endpoint. Defaults to `127.0.0.1:52736`; env: `CECLI_ENDPOINT`. |
| `--connection-name text` | Send a diagnostic name after each connection; env: `CECLI_CONNECTION_NAME`. |
| `--timeout duration` | Set the per-network-operation deadline. Defaults to `30s`; env: `CECLI_TIMEOUT`. |
| `--human` | Render terminal-oriented text. |
| `--agent` | Force the default structured JSON mode. |
| `--pretty` | Indent JSON output. |
| `--quiet`, `-q` | Suppress stdout and stderr; use the exit code as the result. |
| `--fields path,...` | Return selected command-data fields; dotted paths are supported; env: `CECLI_FIELDS`. |
| `--dry-run` | Preview destructive operations without connecting or mutating. |
| `--help`, `-h` | Return full or exact command-specific help as JSON by default. |
| `--brief` | Return the embedded agent business card. |
| `--version` | Return version, commit, and build date. |

## Agent discovery and output shaping

Help is machine-readable by default. A command-local request uses the longest exact catalog match and returns only that command contract:

```bash
cecli memory scan --help --pretty | jq '.data.commands[0]'
cecli debug context set --help --pretty | jq '.data.commands | length'
```

The help manifest includes `schema_version`, the CLI version, typed parameters, defaults, enums, conditional constraints, and success/error envelope schemas. Alternative inputs such as `--pattern` versus `--value` are individually marked optional and described by command constraints rather than falsely reported as universally required.

Project only the data needed by an agent:

```bash
cecli version --fields name,version
CECLI_FIELDS=pid,architecture cecli process info --pid 4242
```

Missing selections do not fail the command; they are listed in `meta.missing_fields`. `--fields` is JSON-only and cannot be combined with `--human` or `--quiet`.

Use quiet mode for probes where the exit code is authoritative:

```bash
cecli self-check --quiet
echo $?
```

Generated Bash, Zsh, and Fish completions include nested subcommands and command-specific flags:

```bash
source <(cecli completion bash --human)
```

## Server

### `cecli server info`

Returns the `ceserver` protocol number, version string, endpoint, and ABI family.

```bash
cecli server info --endpoint 192.168.1.50:52736 --pretty
```

### Server paths and runtime options

```bash
cecli server path
cecli server options list --human
cecli server options get --name optMSO
cecli server options set --name optMSO --value 2 --dry-run
cecli server options set --name optMSO --value 2 --yes
```

Option changes are read back and verified. They fail closed without `--yes`.

### Connection naming and termination

Set the diagnostic name associated with one connection, or automatically name every connection with the global option:

```bash
cecli server connection-name --name ci-worker-1
cecli server info --connection-name ci-worker-1
```

Remote termination has no server response and disconnects all clients. Preview first and execute only against an authorized endpoint:

```bash
cecli server terminate --dry-run
cecli server terminate --yes
```

## Processes

### `cecli process list`

```bash
cecli process list --filter game --limit 50
```

### `cecli process info`

```bash
cecli process info --pid 4242
```

Returns the target architecture plus module and thread counts.

### `cecli process speed`

```bash
cecli process speed --pid 4242 --speed 0.5 --dry-run
cecli process speed --pid 4242 --speed 0.5 --yes
```

This requires the relevant `ceserver` extension support on the target.

## Modules

```bash
cecli module list --pid 4242 --filter libmono
```

Load a module already present on the target filesystem:

```bash
cecli module load --pid 4242 --path /tmp/plugin.so --dry-run
cecli module load --pid 4242 --path /tmp/plugin.so --yes
```

Load with a known target-side `dlopen` address, or inject the upstream ceserver extension:

```bash
cecli module load-ex --pid 4242 --dlopen 0x7F0012345000 --path /tmp/plugin.so --dry-run
cecli module load-ex --pid 4242 --dlopen 0x7F0012345000 --path /tmp/plugin.so --yes
cecli module extension-load --pid 4242 --dry-run
cecli module extension-load --pid 4242 --yes
```

All module and extension loads require `--yes` and accept a no-connect `--dry-run` preview.

## Threads

```bash
cecli thread list --pid 4242
cecli thread create --pid 4242 --start 0x70001000 --parameter 0x70002000 --dry-run
cecli thread create --pid 4242 --start 0x70001000 --parameter 0x70002000 --yes
cecli thread close --handle 73 --yes
```

Thread suspension is represented by the upstream suspend count. These commands require an already-active `ceserver` debug session, such as a long-running `debug trace` in another terminal:

```bash
cecli thread suspend --pid 4242 --tid 4243 --dry-run
cecli thread suspend --pid 4242 --tid 4243 --yes
cecli thread resume --pid 4242 --tid 4243 --yes
```

The result reports `suspend_count`; zero is a successful final resume, while a negative upstream result is treated as a protocol failure.

## Debugger events

`debug trace` owns one debugger connection for a bounded command lifetime. It attaches, collects no more than `--events`, waits no longer than `--event-timeout` for each event, continues every received event, and detaches when the connection closes. `--event-timeout` must be shorter than the global `--timeout` so network framing has a larger deadline than the server wait.

```bash
cecli debug trace --pid 4242 --events 20 --event-timeout 2s --dry-run
cecli debug trace --pid 4242 --events 20 --event-timeout 2s --yes
```

Continuation modes are:

| Value | Behavior |
|---|---|
| `auto` | Default. Ignore virtual create events, `SIGTRAP`, and `SIGSTOP`; deliver other signals. |
| `deliver` | Deliver the original signal when the target continues. |
| `ignore` | Suppress the signal when the target continues. |
| `single-step` | Continue through the upstream single-step path. |

Use `--continue deliver|ignore|single-step` only when the target’s signal semantics are understood. Every trace requires `--yes`; `--dry-run` does not connect.

## Hardware breakpoints

Breakpoint commands operate on an already-active `ceserver` debug session. Set `--tid -1` to apply the requested debug register to all session threads.

```bash
cecli debug breakpoint set \
  --pid 4242 --tid -1 --register 0 \
  --address 0x7FF612340120 --kind execute --size 1 \
  --dry-run

cecli debug breakpoint set \
  --pid 4242 --tid -1 --register 0 \
  --address 0x7FF612340120 --kind execute --size 1 \
  --yes

cecli debug breakpoint remove \
  --pid 4242 --tid -1 --register 0 --yes
```

Breakpoint kinds are `execute`, `write`, `read`, and `access`; sizes are `1`, `2`, `4`, or `8`. Use `--watchpoint` when removing a data watchpoint. Hardware capabilities vary by architecture and are reported by the create-process event at the start of a trace.

## Thread contexts

Context reads return the complete packed upstream blob as standard base64 plus its declared size, type code, and SHA-256 digest:

```bash
cecli debug context get --pid 4242 --tid 4243 --pretty
```

Context writes validate the embedded little-endian size header before connecting, temporarily attach through the debugger path, require confirmation, and can compare a read-back blob:

```bash
cecli debug context set --pid 4242 --tid 4243 \
  --base64 "$CONTEXT_BASE64" --dry-run

cecli debug context set --pid 4242 --tid 4243 \
  --base64 "$CONTEXT_BASE64" --yes --verify
```

`--hex` is an alternative to `--base64`. Register decoding is intentionally not guessed: the context layout depends on the target architecture, compatibility mode, and the upstream `CONTEXT.type` field.

## Memory regions

```bash
cecli memory regions --pid 4242
cecli memory regions --pid 4242 --paged-only --no-shared --human
```

Region records include base address, size, Cheat Engine protection code, normalized permissions, and mapping type.

## Memory reads

Read raw bytes as hexadecimal:

```bash
cecli memory read --pid 4242 --address 0x7FF612340000 --size 64
```

Decode a fixed-width typed value; size is inferred:

```bash
cecli memory read --pid 4242 --address 0x7FF612340120 --format typed --type f32
```

Supported types: `u8`, `i8`, `u16`, `i16`, `u32`, `i32`, `u64`, `i64`, `f32`, `f64`, `utf8`, `utf16le`, and `hex`.

## Exact scans

Scan a byte pattern with whole-byte wildcards:

```bash
cecli memory scan --pid 4242 --pattern "48 8B ?? FF" --start 0x1000 --end 0x7FFFFFFFFFFF --limit 1000
```

Scan a typed value:

```bash
cecli memory scan --pid 4242 --type i32 --value 100 --alignment 4 --protection writable
```

The default scanner enumerates regions through `CMD_VIRTUALQUERYEXFULL` and reads them in bounded chunks. It does not depend on server-side scanner availability.

For explicit compatibility testing of `CMD_AOBSCAN`, invoke the patched bundled server implementation. This path requires confirmation because older unpatched servers contain an unsafe experimental implementation:

```bash
cecli memory aobscan --pid 4242 --pattern "48 8B ?? FF" \
  --start 0x1000 --end 0x7FFFFFFFFFFF --alignment 4 --dry-run

cecli memory aobscan --pid 4242 --pattern "48 8B ?? FF" \
  --start 0x1000 --end 0x7FFFFFFFFFFF --alignment 4 --yes
```

The bundled compatibility patch bounds pattern size, validates ranges and alignment, handles chunk overlap safely, limits match storage, and prevents negative or oversized response counts. Prefer `memory scan` unless the server-side packet itself is under test.

## Memory writes

Preview first:

```bash
cecli memory write --pid 4242 --address 0x7FF612340120 --type i32 --value 999 --dry-run
```

Execute and verify:

```bash
cecli memory write --pid 4242 --address 0x7FF612340120 --type i32 --value 999 --yes --verify
```

Raw bytes are accepted with `--hex`. The command never prompts; it fails with exit code `2` unless `--yes` is supplied.

## Allocation and protection

```bash
cecli memory alloc --pid 4242 --size 4096 --protection rw --dry-run
cecli memory alloc --pid 4242 --size 4096 --protection rw --yes
cecli memory protect --pid 4242 --address 0x5000 --size 4096 --protection rx --yes
cecli memory free --pid 4242 --address 0x5000 --size 4096 --yes
```

Exact page protections are `noaccess`, `r`, `rw`, `rc`, `x`, `rx`, and `rwx`.

## Remote filesystem

```bash
cecli remote pwd
cecli remote ls --path /tmp --human
cecli remote stat --path /tmp/sample.bin
cecli remote get --remote /tmp/sample.bin --local ./sample.bin --dry-run
cecli remote get --remote /tmp/sample.bin --local ./sample.bin --yes
cecli remote put --local ./plugin.so --remote /tmp/plugin.so --dry-run
cecli remote put --local ./plugin.so --remote /tmp/plugin.so --yes
cecli remote mkdir --path /tmp/cecli --yes
cecli remote chmod --path /tmp/plugin.so --mode 0755 --yes
cecli remote rm --path /tmp/plugin.so --yes
cecli remote cd --path /tmp --yes
```

Downloads and uploads require `--yes`; `--dry-run` performs no connection or local/remote write. Downloads refuse to overwrite an existing local file unless `--force` is supplied. Uploads are limited to 256 MiB.

## Named pipes

```bash
cecli pipe open --name ceserver-extension --timeout-ms 1000
cecli pipe write --handle 81 --text hello --timeout-ms 1000 --dry-run
cecli pipe write --handle 81 --text hello --timeout-ms 1000 --yes
cecli pipe read --handle 81 --size 4096 --timeout-ms 1000
cecli pipe close --handle 81 --yes
```

Pipe reads and writes are bounded by explicit sizes. Handles are remote ceserver handles and should be closed when no longer needed.

## Symbols

```bash
cecli symbol list --path /tmp/sample-game
cecli symbol list --path /tmp/sample-game --file-offset 4096 --filter update --module-base 0x70000000
```

The client validates and decompresses the upstream zlib symbol response, applies optional name filtering, and can report module-relative addresses against `--module-base`.

## Protocol coverage boundary

`cecli` covers every non-obsolete command dispatched by the bundled server. Legacy process/module iteration and virtual-query packets are used internally or superseded by richer variants rather than duplicated as raw CLI verbs. `CMD_STOPDEBUG`, `CMD_PTRACE_MMAP`, and `CMD_COMMANDLIST2` are defined upstream but have no dispatch handler. Debug traces therefore detach through connection cleanup.

## Agent skills

```bash
cecli skills
cecli skills memory-inspection
```

## Local feedback

```bash
cecli issue create --type bug --message "Unexpected scan response" --context '{"command":"memory scan"}' --exit-code 1
cecli issue list --status open
cecli issue show CECLI-20260726T120000Z-1234abcd
cecli issue transition --status resolved CECLI-20260726T120000Z-1234abcd
```

Issues are stored locally under the platform user configuration directory, or under `CECLI_STATE_DIR` when set.

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | Success |
| `1` | General or protocol failure |
| `2` | Invalid or missing input; confirmation required |
| `10` | `ceserver` unreachable |
| `20` | Resource not found |
| `30` | Conflict or write verification failure |
| `130` | Interrupted by signal |
