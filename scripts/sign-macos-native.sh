#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "macOS native signing is available only on macOS" >&2
  exit 1
fi

binary_path="${1:-bin/cecli}"
if [[ ! -f "$binary_path" ]]; then
  echo "binary not found: $binary_path" >&2
  echo "run 'make build' first" >&2
  exit 1
fi

identity="${MACOS_SIGN_IDENTITY:-}"
if [[ -z "$identity" ]]; then
  identities="$(security find-identity -v -p codesigning 2>/dev/null || true)"
  identity="$(printf '%s\n' "$identities" | awk -F '"' '/Apple Development:/{print $2; exit}')"
  if [[ -z "$identity" ]]; then
    identity="$(printf '%s\n' "$identities" | awk -F '"' '/Developer ID Application:/{print $2; exit}')"
  fi
fi
if [[ -z "$identity" ]]; then
  echo "no Apple Development or Developer ID Application identity was found" >&2
  echo "install a signing identity or set MACOS_SIGN_IDENTITY explicitly" >&2
  exit 1
fi

repository_root="$(cd "$(dirname "$0")/.." && pwd)"
codesign \
  --force \
  --sign "$identity" \
  --entitlements "$repository_root/build/macos-debugger.entitlements" \
  "$binary_path"
codesign --verify --strict "$binary_path"
echo "signed $binary_path for native macOS process access with: $identity" >&2
