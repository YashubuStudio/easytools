# EasyTools

Read this in [Japanese](README.ja.md).

EasyTools wraps legacy command-line programs and exposes them as HTTP APIs. It ships with a desktop GUI that lets you register, edit and test tools while controlling an embedded HTTP server. Settings can be imported and exported as YAML.

## Features
- Register existing scripts or binaries and expose them through configurable HTTP endpoints.
- Desktop GUI (Fyne) to add/edit tools, start/stop the server and run quick tests.
- Safety options per tool: timeouts, environment variable whitelist, stdout/stderr size caps and optional stdin.
- Built-in log viewer and test console for immediate feedback.
- Import/export configuration as `tools.yaml` for easy reuse.

## Build
Requires Go 1.24.5 and the Fyne GUI toolkit.

### Windows
```bash
go mod tidy
go build -o easytools ./cmd/easytools
```

### Linux
```bash
go mod tidy
go build -o easytools ./cmd/easytools
```

## Run

```bash
./easytools
```

Launch the binary without flags (double-click on desktop platforms) to open the GUI. When EasyTools detects a headless environment (for example SSH sessions without `DISPLAY`) it automatically falls back to CLI mode and starts the HTTP server instead of launching the GUI. Use the following flags for manual control:

```bash
./easytools --server           # start only the HTTP server
sudo ./easytools --webui       # start the server and enable the configuration Web UI on 127.0.0.1:18080
./easytools --config /path/to/tools.yaml --addr :9090
```

The CLI reads and writes `tools.yaml` (same format as the GUI) and honours overrides passed on the command line. The Web UI served by `--webui` exposes a lightweight editor for the YAML configuration and is intended for administrative sessions.

Configure the server address and paths, register tools and click **Start Server** in the GUI or use the CLI mode. When running, the following endpoints become available (defaults shown):

| Method | Path                     | Description                                 |
|--------|--------------------------|---------------------------------------------|
| GET    | `/v1/healthz`            | Health check                                |
| GET    | `/v1/tools`              | List registered tools                       |
| GET    | `/v1/tools/{group}/{name}` | Execute a tool when no API key             |
| POST   | `/v1/run`                | Execute a tool                              |
| POST   | `/v1/reload`             | Reload configuration                        |
| GET    | `/v1/mcp/package`        | Return an MCP manifest for registered tools |
| POST   | `/v1/mcp/run`            | Execute a tool via the MCP request format   |

Each endpoint path is configurable via the `paths` section in the server configuration. Defaults and roles:

- **`base_path`** (`/v1`): prefix added to all endpoints.
- **`paths.tools`** (`/tools`): `GET` lists all registered tools; `GET /{group}/{name}` runs a tool when no API key is set.
- **`paths.run`** (`/run`): `POST` executes a tool specified in the JSON request body.
- **`paths.reload`** (`/reload`): `POST` reloads `tools.yaml` without restarting the server.
- **`paths.health`** (`/healthz`): `GET` health probe returning server status.
- **`paths.mcp_package`** (`/mcp/package`): `GET` manifest describing every tool as an MCP package.
- **`paths.mcp_invoke`** (`/mcp/run`): `POST` endpoint that accepts the MCP invocation payload and returns a single consolidated response.

Example request:

```bash
curl -X POST 'http://localhost:8080/v1/run' \
  -H 'X-API-Key: devkey' \
  -d '{"tool":"echo","params":{"msg":"hello"}}'
```

MCP request and response example:

```bash
curl -X POST 'http://localhost:8080/v1/mcp/run' \
  -H 'X-API-Key: devkey' \
  -d '{
        "name": "echo",
        "input": {
          "params": {"msg": "hello"}
        }
      }'
```

The server returns a single JSON object:

```json
{
  "name": "echo",
  "success": true,
  "output": {
    "command": ["/usr/bin/echo", "hello"],
    "exit_code": 0,
    "stdout": "hello\n",
    "stderr": "",
    "duration_ms": 5,
    "timed_out": false,
    "started_at": "2025-09-10T07:42:22Z",
    "ended_at": "2025-09-10T07:42:22Z"
  }
}
```

Fetch the MCP package manifest at:

```bash
curl -X GET 'http://localhost:8080/v1/mcp/package' -H 'X-API-Key: devkey'
```

If no API key is configured, a tool can be invoked directly:

```bash
curl http://localhost:8080/v1/tools/echo
```

## Application Window

The GUI is split into two main tabs to manage the server and registered tools.

### Server / API

- **Server Settings** – left side fields for address, base path, endpoint paths, API key, a CORS toggle and origin field. Click **Start Server** or **Stop Server** to control the embedded HTTP server and see the current status at the top.
- **Test Console** – right side panel where you can choose a tool, provide JSON parameters or environment variables and run quick requests against the `/run` endpoint. Results are shown beneath the button.
- **Server Logs** – bottom area streaming recent log output for easy debugging.

### Tools (Registry)

- **Tool Form** – left third form to register or edit a tool. Enter name, group, cmd (executable path), args (comma-separated; tokens like `{{msg}}` are replaced with `Params`), working directory, environment variables and safety limits. Buttons allow adding, saving or deleting entries as well as importing or exporting configuration as YAML. Changes are automatically saved to `tools.yaml`.
- **Working Directory** – set `Workdir` to run the command in a specific folder. Commands that depend on location, such as `git`, should have this set to the target repository. If left empty the EasyTools process directory is used.
- **Tool List** – center accordion listing tools grouped by their `Group` value for quick selection.
- **Quick CMD** – right third panel to run a selected tool immediately by supplying JSON parameters, environment or stdin and viewing the HTTP response. Intended for manual testing of individual tools.
  - `Params (JSON)` replaces tokens like `{{name}}` in the tool's `Args`.
  - `Env (JSON)` sets environment variables; only keys listed in `AllowEnv` are applied.
  - `Stdin` sends text to the tool's standard input when `Allow Stdin` is enabled.
  - Example: with `Cmd: /usr/bin/echo` and `Args: ["{{msg}}"]`, entering `{"msg":"hello"}` runs `/usr/bin/echo hello`.

### Example: wrapping git commands

GitHub users can expose familiar git operations like `git status` through an HTTP endpoint:

```yaml
tools:
  repo-status:
    cmd: git
    args: ["status", "--short"]
    workdir: /path/to/repository
```

Select `repo-status` in **Quick CMD** and send a request to `/run` to retrieve the repository status.

## License
Personal use is free of charge and at your own risk. Please contact the author for commercial use. Bug reports and other feedback are welcome. See [LICENSE](LICENSE) for details.

