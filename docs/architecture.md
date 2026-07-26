# Architecture

## Components

```text
AI agent / shell user
        |
        v
cmd/cecli + internal/cli
        |  JSON contracts, validation, dry-run, human rendering
        +---------------- native ----------------+
        |                                        |
        v                                        v
internal/localbridge                     internal/ceserver
Windows APIs / macOS Mach APIs           packed Cheat Engine protocol
        |                                        |
        v                                        v
local authorized process                 ceserver or cebridge
                                                 |
                                                 v
                                         remote authorized process
```

## Design decisions

- **Protocol reuse over reimplementation:** the CLI translates the packed structures defined in `Cheat Engine/ceserver/ceserver.h`, `api.h`, and `networkInterface.pas`.
- **One command surface, two transports:** `--native` runs an in-process protocol bridge backed by Windows or macOS process APIs; `--endpoint` uses the existing TCP client unchanged.
- **Stateless by default:** remote process and snapshot handles are opened and closed within one invocation. `debug trace` is the deliberate exception: it owns one bounded debugger connection for the command lifetime and detaches when that connection closes.
- **JSON by default:** stdout remains machine-readable data; structured failures go to stderr with stable codes and non-zero exit status.
- **Fail-closed writes:** memory writes require `--yes`; `--dry-run` avoids any network connection; `--verify` performs a read-back comparison.
- **Portable scanner by default:** exact scans enumerate memory regions and stream bounded reads. The experimental server-side AOB packet is separately exposed behind explicit confirmation for protocol-completeness testing.
- **Debugger lifetime safety:** event traces require an event count, per-event timeout, explicit confirmation, and deterministic continuation policy. Breakpoint and thread suspend commands clearly require an already-active debug session.
- **Raw-context integrity:** context writes accept a complete upstream blob only, verify its packed size header, require `--yes`, and can perform a read-back comparison.
- **No privileged unit-test dependency:** a fake TCP `ceserver` verifies packet framing and responses across discovery, memory, mutations, remote administration, debug events, breakpoints, contexts, pipes, and symbols.

## Scope

Remote mode covers every non-obsolete command dispatched by the bundled upstream `ceserver`: discovery, regions, reads, client-side and server-side exact scans, writes, allocation, protection, module and extension loading, speed control, connection naming, guarded server termination, server options, remote files, symbols, named pipes, remote threads, bounded event tracing, hardware breakpoints, thread suspend counts, and raw contexts.

Native macOS and Windows mode covers target identity, process discovery, architecture, memory regions, bounded reads, portable exact scans, and guarded writes. The standalone `cebridge` command exposes that same native subset over TCP for explicitly configured cross-machine or VM workflows.

Legacy packet variants already superseded by richer commands are implemented internally where needed rather than exposed as duplicate CLI verbs. `CMD_STOPDEBUG`, `CMD_PTRACE_MMAP`, and `CMD_COMMANDLIST2` are defined upstream but not dispatched, so they have no working server operation to call. Debug traces detach through connection cleanup rather than a nonexistent stop-debug handler.
