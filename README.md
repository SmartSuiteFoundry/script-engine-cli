# sse — SmartSuite Script Engine CLI

Command-line client for the **Script Management API**: script CRUD, execution, run history, and log retrieval. HTTP paths, query parameters, request bodies, and security match **[docs/openapi.yaml](docs/openapi.yaml)** (OpenAPI 3). [docs/ENGINE_README.md](docs/ENGINE_README.md) describes broader system context.

## Requirements

- Go **1.22+** (to build from source)
- **Linux**, **macOS**, or **Windows** (or any platform Go supports). Release archives target **linux** / **darwin** / **windows** on **amd64** and **arm64** where applicable.

## Install

From a clone of this repository:

```bash
go build -o sse ./cmd/sse
# optional: embed version at link time
go build -ldflags "-X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o sse ./cmd/sse
```

Or install onto your `PATH`:

```bash
go install ./cmd/sse
```

**Windows (native):** install [Go for Windows](https://go.dev/dl/), then in PowerShell or `cmd` from the repo root:

```powershell
go build -o sse.exe .\cmd\sse
# or put on PATH:
go install .\cmd\sse
```

Use **`sse configure`** from **Windows Terminal**, **PowerShell**, or **cmd** (console TTY required for hidden password input). **`scripts\install.ps1`** installs a release tarball; see **[docs/DISTRIBUTION.md](docs/DISTRIBUTION.md)**.

Prebuilt cross-compiled binaries can be produced with:

```bash
make cross-linux cross-darwin cross-windows   # outputs under dist/ (Windows builds are *.exe)
```

**Customer installs:** versioned `.tar.gz` archives, checksums, and a small `install.sh` are described in **[docs/DISTRIBUTION.md](docs/DISTRIBUTION.md)** (similar goals to AWS-style installers: one URL per platform + verification; for a Go binary we use tarballs instead of a fat bundle). Optional **[.goreleaser.yaml](.goreleaser.yaml)** automates GitHub Releases and the same archive layout.

## Base URL

`--base-url` / `SSE_BASE_URL` must be the **API root** as in OpenAPI `servers`, including the **`/v1/scripting`** segment, **without** a trailing slash. Examples from the spec:

- `https://hotfix.ss-stage.com/v1/scripting`
- `https://dev.ss-stage.com/v1/scripting`
- `https://stage.ss-stage.com/v1/scripting`

Using only `https://…/scripting` (missing **`v1`**) hits the SmartSuite web app and returns HTML, not the API. The CLI **auto-corrects** a base URL whose path is exactly `/scripting` to **`/v1/scripting`** when resolving config.

The CLI calls paths such as **`/scripts`**, **`/scripts/{scriptId}/execute`**, **`/runtimes`**, relative to that root (not `/api/v1/...`).

## Authentication

Per OpenAPI `security` (`TokenAuth` + `AccountId`):

| Header | Meaning |
|--------|---------|
| `Account-Id` | Workspace / account id (`--account-id`, `SSE_ACCOUNT_ID`, config `account_id`) |
| `Authorization` | **`ApiKey` + space + secret** by default |

Credential flags (**use only one**):

- **`--token`** / **`SSE_TOKEN`** / config **`token`**
- **`--api-key`** / **`SSE_API_KEY`** / config **`api_key`** (same wire format as `token`; second name for convenience)

The CLI sets:

```http
Authorization: ApiKey <your-secret>
```

unless the value already starts with **`Bearer`** or **`ApiKey`** (case-insensitive), in which case it is sent unchanged. That allows JWT-style gateways to keep using `Bearer …` if your deployment requires it.

## Configuration

Precedence (**highest wins**): flags → environment → stored credentials and file fields → defaults.

For **`token`** / **`api_key`**, “stored credentials” means: if the OS secret store has an entry for your resolved config file path, that value is used **instead of** `token` / `api_key` lines in the YAML (so the keyring wins over stale plaintext until you save again). Otherwise the file is read as before.

| Purpose | Flag | Environment variable | Config key |
|---------|------|----------------------|------------|
| API root URL | `--base-url` | `SSE_BASE_URL` | `base_url` |
| Workspace id | `--account-id` | `SSE_ACCOUNT_ID` | `account_id` |
| Primary secret | `--token` | `SSE_TOKEN` | `token` (often stored outside the file; see below) |
| Alternate secret | `--api-key` | `SSE_API_KEY` | `api_key` (often stored outside the file; see below) |
| Config file | `--config` | `SSE_CONFIG` | — |
| Default output (`json` / `pretty`) | `--output` | — | `output` (file only; used when `--output` is not passed) |

Default config file: **`$XDG_CONFIG_HOME/sse/config.yaml`**, or **`~/.config/sse/config.yaml`**. Non-secret fields and fallback secrets are written with mode **0600**.

### Secrets storage (OS keyring)

`sse configure` and `sse config set` try to keep **tokens and API keys out of the config file** by storing them in the platform secret service:

| Platform | Store |
|----------|--------|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service (DBus), e.g. **GNOME Keyring** / **KWallet** via **libsecret** |

- **Service name in the store:** `sse-cli`.
- **Scope:** entries are keyed to the **absolute path** of the config file in use, so different `--config` / `SSE_CONFIG` paths get separate credentials.
- **`sse config get`** loads secrets from the store when present (masked unless `--show-secrets`).
- **Migration:** existing plaintext `token` / `api_key` in YAML still work. The next successful save via **`sse configure`** or **`sse config set`** rewrites the file **without** those fields and copies the secret into the keyring when the store is available.
- **Fallback:** if the secret store cannot be used (common on **minimal Linux** or **headless/CI** without a session bus), the CLI **writes secrets into the config file** (still **0600**) and prints a **warning** to stderr. For automation, prefer **`SSE_TOKEN`** / **`SSE_API_KEY`** so nothing is written to disk by those commands.

### Interactive configuration (`sse configure`)

Similar to **`aws configure`**, this walks through prompts (with **Enter to keep** the value shown in brackets). **API keys and tokens are read with hidden input** when stdin is a TTY. It updates the same config path as `sse config set`: non-secret fields go to YAML; secrets go to the **OS keyring** when possible (see **Secrets storage**). Requires an interactive terminal (not CI / not piped stdin); for automation use `sse config set` or environment variables instead.

Prompts cover: **API base URL**, **Account-Id**, **authentication** (API key vs token), and **default output** (`json` or `pretty`). The default output is stored as config key **`output`** and used when you do **not** pass **`--output`** on the command line.

```bash
sse configure
```

### Config commands

```bash
sse config path
sse config get              # merges YAML with OS-stored secrets (masked by default)
sse config get token --show-secrets
sse config set base_url https://hotfix.ss-stage.com/v1/scripting
sse config set account_id spyv9knb
sse config set token 'your-api-key-secret'
sse config set output json
```

## Global flags

```text
--base-url
--account-id
--token
--api-key
--config
--output pretty|json    # default: indented JSON; json = compact
```

## Commands

### Scripts

| Command | Description |
|---------|-------------|
| `sse scripts list [--cursor …] [--page-size N]` | List scripts (`page_size` 1–100 when set) |
| `sse scripts get <scriptId>` | Get one script |
| `sse scripts create -f path` | Create (`ScriptCreateRequest`); `-f -` or pipe JSON |
| `sse scripts update <scriptId> -f path` | Update (`ScriptUpdateRequest`) |
| `sse scripts delete <scriptId>` | Delete |
| `sse scripts execute <scriptId> [flags] [-f path]` | Execute (`ExecuteRequest`): see below |

**Execute** builds a JSON object with required fields **`mode`** and **`trigger_type`** (OpenAPI `ExecuteRequest`):

- **`--mode`** — `sync` or `async` (default **`sync`**).
- **`--trigger-type`** — `http`, `scheduled`, or `manual` (default **`manual`**).
- **`--caller-ip`** — optional; sets `caller_ip` if non-empty.
- **`-f` / stdin** — optional JSON merged in; any `mode`, `trigger_type`, `payload`, or `caller_ip` in the file overrides the defaults above.

### Runs

| Command | Description |
|---------|-------------|
| `sse runs list <scriptId> [--cursor …] [--page-size N]` | List runs |
| `sse runs get <scriptId> <runId>` | Get one run |
| `sse runs logs <scriptId> <runId>` | `GET …/logs` → `url` field → download log body |
| `sse runs logs … --url-only` | Print presigned URL only |
| `sse runs logs … -w path` | Write logs to file (`--write`) |

### Runtimes

| Command | Description |
|---------|-------------|
| `sse runtimes list` | `GET /runtimes` |
| `sse runtimes libraries <runtime>` | `GET /runtimes/{runtime}/libraries` |

## Examples

```bash
export SSE_BASE_URL=https://hotfix.ss-stage.com/v1/scripting
export SSE_ACCOUNT_ID=spyv9knb
export SSE_TOKEN='your-api-key'

sse scripts list
sse scripts list --page-size 50
sse scripts get qa-script-20260413092249
sse scripts execute qa-script-20260413092249 --mode sync --trigger-type manual --caller-ip 127.0.0.1
sse scripts execute qa-script-20260413092249 -f ./payload.json   # file may include "payload": { ... }

sse runs list qa-script-20260413092249
sse runs logs qa-script-20260413092249 <run-uuid> -w ./run.log
```

## API payloads

Schemas and enums are defined in **[docs/openapi.yaml](docs/openapi.yaml)** (e.g. `ScriptCreateRequest`, `ExecuteRequest`, `ScriptListResult.next_cursor`).

## Errors

Responses use `ErrorEnvelope` (`error.code`, `error.message`). The CLI exits non-zero and prints the error to stderr.

If you see an **HTML error page** instead of JSON, the URL is often wrong (missing `/v1/scripting` on `--base-url`), or the gateway expects JSON client headers. This CLI sends **`Accept: application/json`**, **`Content-Type: application/json`**, and **`User-Agent: sse-cli/1.0`** on management API calls so behavior matches typical `curl` examples.

## Development

```bash
make test
make vet
make build              # writes $(pwd)/sse by default (see Makefile SSE_OUTPUT)
make build SSE_OUTPUT=/tmp/sse   # optional: custom output path
```
