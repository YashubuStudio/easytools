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
go build -o easytools ./cmd/legacy-exec-gui
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

## License
MIT

