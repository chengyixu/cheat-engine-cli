#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "Darwin release archives must be built on macOS" >&2
  exit 1
fi

version="${1:?usage: package-darwin-release.sh VERSION [COMMIT] [BUILD_DATE] [OUTPUT_DIR]}"
commit="${2:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
build_date="${3:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
output_directory="${4:-dist-darwin}"
repository_root="$(cd "$(dirname "$0")/.." && pwd)"
temporary_root="$(mktemp -d)"
trap 'rm -rf "$temporary_root"' EXIT

mkdir -p "$output_directory"
ldflags="-s -w -X github.com/chengyixu/cheat-engine-cli/internal/cli.Version=$version -X github.com/chengyixu/cheat-engine-cli/internal/cli.Commit=$commit -X github.com/chengyixu/cheat-engine-cli/internal/cli.BuildDate=$build_date"

for architecture in arm64 amd64; do
  archive_architecture="$architecture"
  clang_architecture="$architecture"
  if [[ "$architecture" == "amd64" ]]; then
    archive_architecture="x86_64"
    clang_architecture="x86_64"
  fi
  stage_directory="$temporary_root/cecli_Darwin_$archive_architecture"
  mkdir -p "$stage_directory/build" "$stage_directory/docs" "$stage_directory/scripts"
  (
    cd "$repository_root"
    CGO_ENABLED=1 GOOS=darwin GOARCH="$architecture" CC="clang -arch $clang_architecture" \
      go build -trimpath -ldflags "$ldflags" -o "$stage_directory/cecli" ./cmd/cecli
  )
  cp "$repository_root/README.md" "$repository_root/CHANGELOG.md" "$repository_root/NOTICE.md" "$repository_root/SECURITY.md" "$stage_directory/"
  cp "$repository_root/docs/command-reference.md" "$stage_directory/docs/"
  cp "$repository_root/build/macos-debugger.entitlements" "$stage_directory/build/"
  cp "$repository_root/scripts/sign-macos-native.sh" "$stage_directory/scripts/"
  chmod +x "$stage_directory/scripts/sign-macos-native.sh"
  tar -C "$stage_directory" -czf "$output_directory/cecli_Darwin_$archive_architecture.tar.gz" .
done

(
  cd "$output_directory"
  shasum -a 256 cecli_Darwin_*.tar.gz >checksums-darwin.txt
)
