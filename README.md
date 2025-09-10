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

```bash
go mod tidy
go build -o easytools.exe ./cmd/legacy-exec-gui
```

## Run

```bash
./easytools
```

Launch the binary to open the GUI. Configure the server address and paths, register tools and click **Start Server**. When running, the following endpoints become available (defaults shown):

| Method | Path          | Description           |
|--------|---------------|-----------------------|
| GET    | `/v1/healthz` | Health check          |
| GET    | `/v1/tools`   | List registered tools |
| POST   | `/v1/run`     | Execute a tool        |
| POST   | `/v1/reload`  | Reload configuration  |

Example request:

```bash
curl -X POST 'http://localhost:8080/v1/run' \
  -H 'X-API-Key: devkey' \
  -d '{"tool":"echo","params":{"msg":"hello"}}'
```

## Application Window

The GUI is split into two main tabs to manage the server and registered tools.

### Server / API

- **Server Settings** – left side fields for address, base path, endpoint paths, API key, a CORS toggle and origin field. Click **Start Server** or **Stop Server** to control the embedded HTTP server and see the current status at the top.
- **Test Console** – right side panel where you can choose a tool, provide JSON parameters or environment variables and run quick requests against the `/run` endpoint. Results are shown beneath the button.
- **Server Logs** – bottom area streaming recent log output for easy debugging.

### Tools (Registry)

- **Tool Form** – left third form to register or edit a tool. Enter name, group, cmd (executable path), args (comma-separated; tokens like `{{msg}}` are replaced with `Params`), working directory, environment variables and safety limits. Buttons allow adding, saving or deleting entries as well as importing or exporting configuration as YAML.
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
MIT

