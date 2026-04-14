# Distributing `sse` to customers

The AWS CLI v2 ships a **platform-specific installer** (a signed bundle that unpacks Python + the CLI). For a small **Go** binary, the usual pattern is simpler: **versioned archives**, **checksums**, and an **install script** (or package managers). This repo supports both a **Makefile** flow and optional **GoReleaser** (common for open-source CLIs).

## What customers need

1. A **base URL** where you host release files (HTTPS CDN, S3 static site, GitHub Releases, etc.).
2. Per release: **tarballs** per OS/arch + **`SHA256SUMS`** for integrity (same idea as many Linux distros and AWS’s published checksums for zip installers).

## Layout produced here

After building archives (see below), `dist/` contains:

| File | Purpose |
|------|--------|
| `sse_<version>_linux_amd64.tar.gz` | Linux x86_64 binary named `sse` at archive root |
| `sse_<version>_linux_arm64.tar.gz` | Linux ARM64 |
| `sse_<version>_darwin_amd64.tar.gz` | macOS Intel |
| `sse_<version>_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `sse_<version>_windows_amd64.tar.gz` | Windows x64; archive root is **`sse.exe`** |
| `sse_<version>_windows_arm64.tar.gz` | Windows ARM64 (e.g. Windows 11 on Arm) |
| `SHA256SUMS` | `shasum -a 256` / `sha256sum` output for the `.tar.gz` files |

Customers extract the archive or run:

- **macOS / Linux:** [`scripts/install.sh`](../scripts/install.sh) — verifies checksum, installs to `~/.local/bin` by default.
- **Windows:** [`scripts/install.ps1`](../scripts/install.ps1) — same layout; needs **PowerShell 5.1+** and **`tar`** (included on Windows 10+). Default install dir: `%USERPROFILE%\.local\bin`.

## Build release artifacts locally

```bash
VERSION=1.0.0 ./scripts/dist-archives.sh
```

Or wire `VERSION` from git tag:

```bash
VERSION="$(git describe --tags --always --dirty | sed 's/^v//')" ./scripts/dist-archives.sh
```

Upload the contents of **`dist/`** for that version to your host under a versioned prefix, e.g. `https://downloads.example.com/sse/v1.0.0/`.

## One-line install (customer)

Host **`install.sh`** next to the tarballs (or serve the raw file from GitHub). Customer runs:

```bash
curl -sSf https://downloads.example.com/sse/v1.0.0/install.sh | bash -s -- \
  --base-url https://downloads.example.com/sse/v1.0.0 \
  --version 1.0.0
```

Optional:

```bash
curl ... | bash -s -- --base-url ... --version 1.0.0 --install-dir /usr/local/bin
# may require sudo for /usr/local/bin
```

Local test without a server:

```bash
VERSION=0.0.0-dev ./scripts/dist-archives.sh
./scripts/install.sh --base-url "file://$(pwd)/dist" --version 0.0.0-dev
```

**Windows (local dist folder):**

```powershell
$env:VERSION = "0.0.0-dev"; make dist-archives   # or set VERSION in Git Bash / WSL
.\scripts\install.ps1 -LocalDist .\dist -Version 0.0.0-dev
```

**Windows (remote):**

```powershell
Set-ExecutionPolicy -Scope Process Bypass   # if needed for unsigned script
.\install.ps1 -BaseUrl "https://downloads.example.com/sse/v1.0.0" -Version "1.0.0"
```

## GoReleaser (optional, AWS-style automation)

[GoReleaser](https://goreleaser.com/) builds the same matrix, generates archives and checksums, and can push **GitHub Releases**, **Homebrew taps**, **apt/yum** repos, etc.—similar end goal to AWS’s multi-channel distribution, with less custom scripting.

- Config: [.goreleaser.yaml](../.goreleaser.yaml) in this repo (snapshot: `goreleaser release --snapshot --clean`).
- CI: on `git tag v*`, run `goreleaser release` with a token that can upload to your release host.

## Other channels (common in the industry)

| Channel | Notes |
|---------|--------|
| **GitHub Releases** | Attach `tar.gz` + `SHA256SUMS`; link `install.sh` in release notes. |
| **Homebrew** | Private or public tap; formula `url` + `sha256` per version. |
| **Docker** | `COPY` binary from multi-stage Go build; good for CI runners. |
| **Signed macOS pkg / notarized app** | Enterprise pattern; more setup than tarballs. |
| **Linux packages** | `.deb` / `.rpm` via GoReleaser or `nfpm`. |

## Security notes

- Serve artifacts **only over HTTPS**.
- Publish **checksums** alongside binaries; `install.sh` verifies them.
- For high assurance, also sign archives (**minisign**, **cosign**, or Apple notarization) and document customer verification—similar in spirit to AWS’s signed installers.
