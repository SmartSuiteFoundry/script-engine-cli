#!/usr/bin/env bash
# Install sse from a release directory (tarballs + SHA256SUMS from scripts/dist-archives.sh).
#
# Example (after uploading dist/* to your CDN):
#   curl -sSf https://downloads.example.com/sse/v1.0.0/install.sh | bash -s -- \
#     --base-url https://downloads.example.com/sse/v1.0.0 \
#     --version 1.0.0
#
# Or clone the repo and run locally:
#   ./scripts/install.sh --base-url file://$PWD/dist --version 0.0.0-dev
#
set -euo pipefail

VERSION=""
BASE_URL=""
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

usage() {
  cat <<'EOF'
Install sse from a release directory (tarballs + SHA256SUMS).

Options:
  --base-url URL   HTTPS (or file://) directory with sse_VERSION_os_arch.tar.gz + SHA256SUMS
  --version VER    Must match tarball names (e.g. 1.0.0)
  --install-dir D  Default: $HOME/.local/bin (or INSTALL_DIR)
EOF
  exit "${1:-1}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) BASE_URL="${2:-}"; shift 2 ;;
    --version) VERSION="${2:-}"; shift 2 ;;
    --install-dir) INSTALL_DIR="${2:-}"; shift 2 ;;
    -h|--help) usage 0 ;;
    *) echo "unknown arg: $1" >&2; usage ;;
  esac
done

if [[ -z "$BASE_URL" || -z "$VERSION" ]]; then
  echo "error: --base-url and --version are required" >&2
  usage
fi

BASE_URL="${BASE_URL%/}"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported CPU: $(uname -m)" >&2; exit 1 ;;
esac

archive="sse_${VERSION}_${os}_${arch}.tar.gz"
sumfile="SHA256SUMS"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

fetch() {
  local url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
  else
    echo "need curl or wget" >&2
    exit 1
  fi
}

# file:// support for local testing
if [[ "$BASE_URL" == file://* ]]; then
  localpath="${BASE_URL#file://}"
  cp "${localpath}/${archive}" "$tmpdir/$archive"
  cp "${localpath}/${sumfile}" "$tmpdir/$sumfile"
else
  fetch "${BASE_URL}/${archive}" "$tmpdir/$archive"
  fetch "${BASE_URL}/${sumfile}" "$tmpdir/$sumfile"
fi

(
  cd "$tmpdir"
  want="$(grep -F "$archive" "$sumfile" 2>/dev/null | head -1 | awk '{print $1}' || true)"
  if [[ -z "$want" ]]; then
    echo "error: ${archive} not found in ${sumfile}" >&2
    exit 1
  fi
  if command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$archive" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$archive" | awk '{print $1}')"
  else
    echo "need shasum or sha256sum to verify archive" >&2
    exit 1
  fi
  if [[ "$got" != "$want" ]]; then
    echo "error: checksum mismatch for $archive" >&2
    exit 1
  fi
)

tar xzf "$tmpdir/$archive" -C "$tmpdir"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmpdir/sse" "$INSTALL_DIR/sse"

echo "Installed sse to $INSTALL_DIR/sse"
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "Add to PATH, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\""
fi
