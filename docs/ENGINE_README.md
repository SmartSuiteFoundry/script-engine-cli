# Script Management API

Configuration, orchestration, and management backend for SmartSuite's Custom Script Integrations feature. Manages the full lifecycle of customer-authored JavaScript scripts — CRUD, validation, secret management, scheduling, execution orchestration, and run history.

## System Context

Custom Script Integrations lets SmartSuite customers run their own JavaScript code as managed serverless functions. Two independently deployable components:

1. **Script Management API** (this service) — shared Go service handling script CRUD, validation, secrets, scheduling, execution, and run history. One deployment serves all workspaces.
2. **Script Runner Lambda** (separate repo) — per-workspace Lambda that executes customer scripts. Pulls scripts from S3 by UUID at invocation time.

## API Endpoints

All endpoints require the `Account-Id` header for workspace scoping.

For the **HTTP paths, query parameters, request/response schemas, and `Authorization: ApiKey …` security** used by SmartSuite gateways (for example `/v1/scripting/…`), use **[openapi.yaml](openapi.yaml)** in this folder as the source of truth. The table below uses a logical `/api/v1` prefix as implemented inside the management service; the deployed URL prefix may differ per environment.

### Scripts

| Method | Path | Description | Status |
|--------|------|-------------|--------|
| `POST` | `/api/v1/scripts` | Create a new script | `201 Created` |
| `GET` | `/api/v1/scripts` | List scripts (cursor-based pagination) | `200 OK` |
| `GET` | `/api/v1/scripts/{id}` | Get a script by ID | `200 OK` |
| `PUT` | `/api/v1/scripts/{id}` | Update a script | `200 OK` |
| `DELETE` | `/api/v1/scripts/{id}` | Delete a script | `204 No Content` |

### Execution

| Method | Path | Description | Status |
|--------|------|-------------|--------|
| `POST` | `/api/v1/scripts/{id}/execute` | Execute a script (sync or async) | `200 OK` / `202 Accepted` |

### Runs

| Method | Path | Description | Status |
|--------|------|-------------|--------|
| `GET` | `/api/v1/scripts/{id}/runs` | List runs for a script | `200 OK` |
| `GET` | `/api/v1/scripts/{id}/runs/{runId}` | Get a specific run | `200 OK` |
| `GET` | `/api/v1/scripts/{id}/runs/{runId}/logs` | Get presigned URL for run logs | `200 OK` / `404` |

### Runtimes

| Method | Path | Description | Status |
|--------|------|-------------|--------|
| `GET` | `/api/v1/runtimes` | List available runtimes | `200 OK` |
| `GET` | `/api/v1/runtimes/{runtime}/libraries` | List pre-installed libraries for a runtime | `200 OK` |

### Error Responses

All errors use a consistent envelope:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "description of the error",
    "details": []
  }
}
```

| HTTP Status | Error Code | Cause |
|-------------|-----------|-------|
| `400` | `VALIDATION_FAILED` | Invalid input or script validation failure |
| `404` | `NOT_FOUND` | Resource not found (or cross-account access) |
| `409` | `CONFLICT` | Duplicate script ID in the same workspace |
| `422` | `BUSINESS_RULE_VIOLATION` | e.g. executing an inactive script |
| `503` | `WORKSPACE_NOT_READY` | Workspace Lambda not active |

## Project Structure

```
├── cmd/
│   └── script-service/              # Entrypoint — app.RunAppV2 handles Lambda + local
│       └── main.go
├── internal/
│   ├── config/                      # Service config (embeds coreconfig.CoreConfig)
│   ├── controller/                  # HTTP handlers (ScriptController, RuntimeController)
│   ├── service/                     # Business logic, validation, orchestration
│   ├── repository/                  # Data access interfaces and implementations
│   │   ├── memory/                  # In-memory implementations (testing)
│   │   └── postgres/                # PostgreSQL implementations (sqlc)
│   ├── model/                       # Domain types (Script, ScriptRun, WorkspaceLambda)
│   ├── middleware/                   # Account-Id extraction, workspace scoping
│   ├── store/                       # sqlc-generated database code
│   └── testutil/                    # Shared test helpers
├── db/
│   ├── migrations/                  # SQL migration files
│   └── queries/                     # sqlc query files
├── docs/
│   └── openapi.yaml                 # OpenAPI 3.0 specification
├── go.mod
├── go.sum
└── Makefile
```

## Architecture

Layered: **Controller → Service → Repository**

- **Controllers** — HTTP handlers that parse requests, delegate to services, and map errors to HTTP status codes
- **Services** — business logic, validation, secret management, schedule management, execution orchestration
- **Repositories** — data access via interfaces; in-memory for testing, PostgreSQL (sqlc) for production
- **Middleware** — extracts `Account-Id` header, stores in context; all queries auto-scoped to account

AWS clients (S3, Secrets Manager, Lambda, EventBridge Scheduler) are interface-based with mock implementations for testing.

## Development

### Prerequisites

- Go 1.23+
- Docker (for PostgreSQL)
- sqlc (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)

### Commands

```bash
make build            # Build the binary
make run              # Run with hot reload (gow)
make test             # Run all tests with coverage
make lint             # Run golangci-lint
make checks           # Format + lint + test + build

make db-up            # Start PostgreSQL via docker-compose
make db-down          # Stop PostgreSQL
make migrate-up       # Run database migrations
make sqlc-generate    # Regenerate sqlc code from SQL queries

make lambda           # Build for AWS Lambda (linux/arm64)
```

### Running Locally

```bash
make db-up
make migrate-up
make run
```

### Running Tests

```bash
# All tests (in-memory repositories)
make test

# Specific property test
go test -run TestProperty_ScriptCreateReadRoundTrip ./internal/service
```

## Key Design Decisions

- **Cursor-based pagination** with nanosecond-precision timestamps for stable, efficient paging
- **Script content stored in S3** by UUID; metadata in PostgreSQL
- **Secrets never stored raw** — automatically uploaded to AWS Secrets Manager, replaced with `secret://` references
- **Cross-account access returns 404** (not 403) to prevent workspace enumeration
- **On-demand log consolidation** for timed-out Lambda runs — when the Lambda times out before consolidating log chunks, the `GET .../logs` endpoint discovers chunk files in S3, concatenates them in sequence order, writes the consolidated file, and returns a presigned URL
- **22 property-based tests** using `rapid` validate correctness invariants across the entire service layer
- **Dual deployment** — same binary runs as Lambda or standalone HTTP server via `app.RunAppV2`
