# Product marketing context: Cheat Engine CLI

## Product overview

- **One-line description:** A JSON-first command-line client for authorized process inspection, memory scanning, remote administration, and bounded debugger workflows through Cheat Engine `ceserver`.
- **Category:** Developer tool, process memory inspector, `ceserver` client, AI-native CLI.
- **Product type:** Open-source command-line software distributed as a single Go binary plus source.
- **Business model:** Free public repository; no paid tier defined.

## Target audience

- Developers automating authorized game/application debugging and modding workflows.
- Security researchers and interoperability engineers inspecting controlled processes.
- Cheat Engine users who need repeatable scripts instead of GUI automation.
- AI-agent builders who require strict JSON, stable exit codes, dry-run support, and self-description.
- English- and Simplified-Chinese-speaking users who want first-party setup and safety guidance without relying on machine translation.

## Jobs to be done

1. Discover a remote target process and inspect its modules, threads, architecture, memory map, and symbols.
2. Read or scan process memory reproducibly from scripts and CI.
3. Preview, confirm, execute, and verify an authorized memory write.
4. Collect debugger events or change low-level target state through explicit event, timeout, confirmation, and verification bounds.
5. Give AI agents a bounded tool contract that fails closed instead of prompting.

## Problems and pain points

- GUI-only workflows are difficult to reproduce and compose.
- Ad hoc protocol scripts frequently get packed field widths, framing, stdout/stderr separation, and safety checks wrong.
- Automation needs explicit failure codes and non-interactive confirmation behavior.
- Existing memory tools often lack an agent-readable self-description and feedback loop.

## Competitive landscape

- **Cheat Engine GUI:** broader interactive feature surface; less suited to shell automation.
- **GameConqueror/scanmem:** native Linux scanning; different workflow and protocol ecosystem.
- **Custom `ptrace` scripts:** flexible but high maintenance and platform-specific.
- **Generic debugger CLIs:** strong debugger semantics but not optimized for Cheat Engine network workflows.

## Differentiation

- Direct compatibility with the packed protocol implemented in upstream Cheat Engine source.
- Coverage of every non-obsolete dispatched `ceserver` command, with duplicate legacy packets kept internal and undispatched IDs documented honestly.
- JSON by default with structured stderr errors, stable exit codes, dotted field projection, and silent probes.
- Typed command-specific help with defaults, enums, conditional constraints, and response-envelope schemas.
- Embedded agent rules/skills on every response.
- Write safety through no-connect dry runs, explicit `--yes`, and read-back verification.
- Bounded debug traces, active-session breakpoint controls, and validated raw context round trips.
- Standard-library Go binary with single-digit-millisecond local startup in release checks.
- A separately installable bilingual agent skill that turns the command surface into an authorization-to-restore workflow.

## Objections and responses

- **“Why not use the GUI?”** Use the GUI for visual exploration; use `cecli` when repeatability, pipes, CI, or agent control matters.
- **“Is this official?”** No. Position it clearly as an independent, protocol-compatible project.
- **“Is exposing memory writes unsafe?”** Writes fail closed, require explicit confirmation, support dry-run, and can verify read-back.
- **“Can I redistribute it?”** The upstream repository has no detected repository-wide license; require a licensing review before combined redistribution.

## Anti-personas

- Users seeking unauthorized access, multiplayer cheating, credential extraction, stealth, persistence, or anti-cheat bypass.
- Teams requiring an authenticated encrypted transport without adding a trusted tunnel or network control.

## Customer language

- “Cheat Engine CLI”
- “ceserver client”
- “command-line memory scanner”
- “script process memory reads”
- “JSON process inspection”
- “AI-agent memory debugging tool”

Avoid claiming “official,” “undetectable,” “anti-cheat bypass,” or unrestricted cross-platform local access.

## Brand voice

Technical, direct, vivid, safety-aware, independent, and evidence-based in both English and Simplified Chinese. Prefer exact commands, limits, protocol sources, and verified capabilities over hype.

## Priority discovery queries

1. cheat engine cli
2. cheat engine command line
3. ceserver client
4. command line memory scanner
5. process memory scanner cli
6. cheat engine automation
7. json memory inspection tool
8. ai agent cli process memory
9. remote process memory reader
10. cheat engine linux server client
11. Cheat Engine 命令行
12. 命令行扫描进程内存
13. macOS 进程内存读取
14. Windows 游戏内存扫描
