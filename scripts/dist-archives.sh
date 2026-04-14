#!/usr/bin/env bash
# Build cross-compiled binaries and versioned tar.gz archives under dist/ (plus SHA256SUMS).
# Usage: from repo root, VERSION=1.2.3 ./scripts/dist-archives.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.0.0-dev}"
export VERSION

# Propagate into `make cross` so -ldflags -X main.Version=... matches release archives.
make cross

rm -f dist/sse_"${VERSION}"_*.tar.gz dist/SHA256SUMS 2>/dev/null || true

for pair in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64; do
  os=${pair%-*}
  arch=${pair#*-}
  if [[ "$os" == "windows" ]]; then
    bin="dist/sse-${pair}.exe"
    exename="sse.exe"
  else
    bin="dist/sse-${pair}"
    exename="sse"
  fi
  if [[ ! -f "$bin" ]]; then
    echo "missing $bin; run make cross first" >&2
    exit 1
  fi
  tmp="$(mktemp -d)"
  cp "$bin" "$tmp/$exename"
  chmod +x "$tmp/$exename" 2>/dev/null || true
  (cd "$tmp" && tar czf "$ROOT/dist/sse_${VERSION}_${os}_${arch}.tar.gz" "$exename")
  rm -rf "$tmp"
done

cd dist
if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 sse_"${VERSION}"_*.tar.gz > SHA256SUMS
elif command -v sha256sum >/dev/null 2>&1; then
  sha256sum sse_"${VERSION}"_*.tar.gz > SHA256SUMS
else
  echo "need shasum or sha256sum to write SHA256SUMS" >&2
  exit 1
fi

echo "Wrote:"
ls -1 "sse_${VERSION}"_*.tar.gz SHA256SUMS
