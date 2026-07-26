---
name: getting-started
description: Select a native or remote target and inspect its available processes.
---

For a local macOS or Windows target, run `cecli --native server info`, then `cecli --native process list`. On macOS, build and sign first with `make sign-macos-native`. For a remote target, use `--endpoint host:port` or `CECLI_ENDPOINT`. JSON is the default output; add `--human` for terminal-oriented tables.
