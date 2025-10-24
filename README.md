# EasyTools

Read this in [Japanese](README.ja.md).

EasyTools is a lightweight framework that accepts agent invocations via the Model Context Protocol (MCP) and executes pre-registered commands inside a sandbox. Incoming JSON payloads are validated and reduced to predefined fields before the matching API is executed. Results go through verification and masking so that sensitive data stays protected even when the tool is embedded in automated workflows.

## Highlights
- **MCP endpoints** – `/v1/mcp/run` executes tools, `/v1/mcp/package` returns descriptors containing name, arguments, constraints, sample requests/responses and optional natural-language explanations.
- **Secure command execution** – Blocks unregistered commands, shell parsing (pipes/redirects), `sudo` and arbitrary process launches. Commands run as an unprivileged user with limits on wall clock time, memory and accessible paths.
- **Fine-grained API registration** – Configure working directory, command, timeout, environment variables and more per tool from the dedicated CLI. Input validation and masking keep the interface predictable for agents.
- **Local network ready** – Optional CORS configuration and API key authentication allow safe exposure on a LAN.
- **Portable Go implementation** – Minimal dependencies and per-OS binaries. Documentation, configuration recipes, MCP schemas and experiment scripts are tracked in this repository with pinned commit IDs for reproducible evaluation.

## Build
Requires Go 1.24.5 and the Fyne GUI toolkit.

```bash
go mod tidy
go build -o easytools ./cmd/easytools
```

## CLI and server startup

```bash
./easytools --server \
  --config /path/to/tools.yaml \
  --addr :8080
```

Launching the binary without flags (double-click on desktop platforms) opens the management GUI. Both GUI and CLI read/write the same `tools.yaml`; command-line flags take precedence.

Per registered API you can configure:

- `cmd` / `args`: executable path and fixed arguments (no shell interpretation)
- `workdir`: working directory for the process
- `timeout`: execution timeout in milliseconds
- `env` / `allow_env`: predefined environment variables and whitelist for request-provided keys
- `stdin`: whether standard input is allowed and its default value
- `limits`: resource limits for time, memory and IO size

## HTTP endpoints

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/mcp/run` | Validates an MCP payload and runs the matching registered API inside the sandbox. |
| `GET` | `/v1/mcp/package` | Returns descriptors with names, arguments, constraints, request/response samples and optional natural-language notes. |
| `POST` | `/v1/run` | Executes a tool using a simplified JSON payload. |
| `GET` | `/v1/tools` | Lists registered tools. |
| `POST` | `/v1/reload` | Reloads `tools.yaml`. |
| `GET` | `/v1/healthz` | Health probe for the server. |

Endpoints inherit `base_path` and can be remapped through the `paths.*` settings. API key authentication uses the `X-API-Key` header. When CORS is enabled the server adds appropriate `Access-Control-Allow-*` headers.

### MCP request example

```bash
curl -X POST 'http://localhost:8080/v1/mcp/run' \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: devkey' \
  -d '{
        "name": "echo",
        "input": {
          "params": {"msg": "hello"}
        }
      }'
```

The response is a single validated JSON document with masked fields where necessary.

### MCP package example

```bash
curl -H 'X-API-Key: devkey' \
  http://localhost:8080/v1/mcp/package
```

The payload contains the API name, argument schema, constraints, sample requests/responses and optional natural-language explanations.

## Sandbox and safety
- Processes run as an unprivileged user; privilege escalation (`sudo`, custom UIDs) is not allowed.
- No shell parsing: only registered binaries are launched directly, so pipes, redirects or subshells are ignored.
- Resource quotas (time, memory, stdout/stderr size) stop runaway executions.
- IO paths are restricted to the locations defined in the registration to prevent writing outside the sandbox.

## Reproducibility
Build instructions, configuration samples, MCP descriptor schemas and evaluation scripts live in this repository. Documentation references pinned commit IDs so experiments can be reproduced across environments. Portable Go binaries are published for major operating systems.

## License
See [LICENSE](LICENSE).
