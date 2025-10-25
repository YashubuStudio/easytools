# EasyTools

Read this in [Japanese](README.ja.md).

EasyTools is a lightweight framework that accepts agent invocations via the Model Context Protocol (MCP) and executes pre-registered commands inside a sandbox. Incoming JSON payloads are validated and reduced to predefined fields before the matching API is executed. Results go through verification and masking so that sensitive data stays protected even when the tool is embedded in automated workflows.

## Highlights
- **MCP endpoints** – `/mcp/run` executes tools, `/mcp/package` returns descriptors containing name, arguments, constraints, sample requests/responses and optional natural-language explanations.
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
| `POST` | `/mcp/run` | Validates an MCP payload and runs the matching registered API inside the sandbox. Also accepts the legacy `/run` payload for compatibility. |
| `GET` | `/mcp/package` | Returns descriptors with names, arguments, constraints, request/response samples and optional natural-language notes. |
| `POST` | `/run` | Executes a tool using a simplified JSON payload. |
| `GET` | `/tools` | Lists registered tools. |
| `POST` | `/reload` | Reloads `tools.yaml`. |
| `GET` | `/healthz` | Health probe for the server. |

Endpoints inherit `base_path` and can be remapped through the `paths.*` settings. API key authentication uses the `X-API-Key` header. When CORS is enabled the server adds appropriate `Access-Control-Allow-*` headers.

### MCP request example

```bash
curl -X POST 'http://localhost:8080/mcp/run' \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: devkey' \
  -d '{
        "name": "echo",
        "input": {
          "params": {"msg": "hello"}
        }
      }'
```

> [!TIP]
> On Windows `cmd.exe`, replace the single quotes and line continuations
> with double quotes and caret (`^`) continuations:
>
> ```cmd
> curl -X POST "http://localhost:8080/mcp/run" ^
>   -H "Content-Type: application/json" ^
>   -H "X-API-Key: devkey" ^
>   -d "{\\
>         \"name\": \"echo\",\\
>         \"input\": {\\
>           \"params\": {\"msg\": \"hello\"}\\
>         }\\
>       }"
> ```
>
> PowerShell users can keep the backtick (`\``) line continuation but should
> keep the single quotes from the Bash example.

The response is a single validated JSON document with masked fields where necessary.

#### `/mcp/run` request contract

- **Method & path**: `POST /mcp/run` (or the remapped `base_path` + `paths.mcp_invoke`).
- **Headers**: Always send `Content-Type: application/json`. When API-key auth is enabled, include `X-API-Key: <value from config>`.
- **Body**: A single JSON object matching the `InvokeRequest` shape.

```json
{
  "name": "<tool name as defined in tools.yaml>",
  "input": {
    "params": {"<template token>": "value", ...},
    "env": {"<allowed env var>": "value", ...},
    "stdin": "string passed to the tool's standard input"
  }
}
```

| Field | Required | Notes |
| --- | --- | --- |
| `name` | ✅ | Key of the registered tool. Leading/trailing whitespace is trimmed; an empty name returns HTTP 400. |
| `input` | optional | Container for extra values. When omitted the tool runs with empty params/env/stdin. |
| `input.params` | conditional | Supplies replacements for `{{token}}` placeholders found in the tool's `args`. Tokens declared with `input.params` and those auto-detected from the template are all required. Missing entries raise HTTP 400. |
| `input.env` | optional | Values for variables allowed by `allow_env` or declared in `input.env`. Sending a key outside the whitelist raises HTTP 400. |
| `input.stdin` | conditional | Mandatory when the tool sets `allow_stdin: true` and `input.stdin.required: true`. Sending stdin while it is disallowed raises HTTP 400. |

##### Template expansion rules

- Every argument is scanned for `{{token}}` placeholders. Each placeholder is replaced with the corresponding `input.params[token]` value.
- Replacement is string-based. If any `{{` remains after substitution the server returns HTTP 400 with `arg template error`.
- Even without an explicit `input.params` definition, all detected template tokens become required inputs.
- Values are converted via `fmt.Sprint`, so non-string values are stringified. Reusing the same token multiple times only requires one entry in `input.params`.

##### Responses and status codes

- Successful executions return `200 OK` with an object containing `name`, `success` and `output`. `output.command` lists the executable path plus arguments.
- Tool failures (non-zero exit code, timeout) still return the structured body but with HTTP status `400 Bad Request`.
- Validation issues or unknown tools return `400 Bad Request` or `404 Not Found`. Invalid API keys return `401 Unauthorized`, and unsupported methods return `405 Method Not Allowed`.

##### Authentication (`X-API-Key`)

- Setting `api_key` in the server config makes the `X-API-Key` header mandatory for every endpoint.
- Missing or incorrect keys yield `401 Unauthorized` with `{"error": "missing/invalid api key"}`.
- When `cors: true` is enabled the server emits `Access-Control-Allow-Headers: Content-Type, X-API-Key` and answers preflight (`OPTIONS`) requests with `204 No Content`.

### MCP package example

```bash
curl -H 'X-API-Key: devkey' \
  http://localhost:8080/mcp/package
```

```cmd
curl -H "X-API-Key: devkey" http://localhost:8080/mcp/package
```

### Simple JSON request example

```bash
curl -X POST 'http://localhost:8080/run' \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: devkey' \
  -d '{
        "tool": "echo",
        "params": {"msg": "hello"}
      }'
```

The server returns `RunResponse`, which includes the executed command, stdout/stderr, exit code and timing metadata.

### Adding a `git pull` command

1. Edit `tools.yaml` (created automatically on first launch) and add a new entry under the `tools` map:

   ```yaml
   tools:
     git_pull:
       cmd: git
       args: ["pull"]
       workdir: /path/to/your/repository
       timeout: 30s
   ```

   Adjust `workdir` to the repository you want to update. Optional keys such as `env`, `allow_env`, `stdin` and `limits` can be added as needed.

2. Reload the configuration via the GUI or send `POST http://localhost:8080/reload` with the correct `X-API-Key` header so the server picks up the new tool.

3. Invoke the tool through the simplified API:

   ```bash
   curl -X POST 'http://localhost:8080/run' \
     -H 'Content-Type: application/json' \
     -H 'X-API-Key: devkey' \
     -d '{"tool": "git_pull"}'
   ```

    Or call it through MCP by specifying `"name": "git_pull"` inside the MCP payload.

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
