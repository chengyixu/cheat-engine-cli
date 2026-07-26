# Security policy

## Supported versions

Security fixes target the latest tagged `cecli` release and the current `master` branch.

## Reporting

Do not include credentials, private memory dumps, exploit payloads, or personal data in public issues. Use a private security advisory in the authorized GitHub repository once it exists, or contact the repository owner through their verified GitHub profile.

## Operational safety

- Use `cecli` only on systems and processes you own or are explicitly authorized to inspect.
- Bind `ceserver` to trusted interfaces and restrict access with host firewalls or authenticated tunnels.
- Treat process memory as sensitive data.
- Preview writes with `--dry-run`, require `--yes`, and prefer `--verify`.
- Do not run remote file, injection, debugger, or termination capabilities without an explicit review of target authorization and rollback behavior.
- Prefer `memory scan`; use `memory aobscan --yes` only when testing the server-side packet against the bundled patched server. Older unpatched servers contain an unsafe experimental scanner.
- Review local paths before `remote put`, `remote get`, or `pipe write --file`; those commands can transfer sensitive files and require explicit confirmation for mutation.
- `cecli` does not invoke a shell for command arguments. Paths, patterns, values, and pipe payloads are parsed locally or transmitted as protocol data, so normal filesystem syntax is not rejected as shell syntax.
