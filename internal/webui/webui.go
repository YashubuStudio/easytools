package webui

import (
	"context"
	"errors"
	"html/template"
	"io"
	"net/http"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/yashubustudio/easytools/internal/appconfig"
	"github.com/yashubustudio/easytools/internal/util"
)

type Server struct {
	srv *http.Server
}

func Start(ctx context.Context, addr, configPath string, logWriter io.Writer) (*Server, <-chan error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := appconfig.Load(configPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		page := pageData{ConfigYAML: mustYAML(cfg)}
		if err := pageTmpl.Execute(w, page); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		body := r.PostFormValue("content")
		if body == "" {
			http.Error(w, "empty payload", http.StatusBadRequest)
			return
		}
		cfg := appconfig.Default()
		if err := yaml.Unmarshal([]byte(body), &cfg); err != nil {
			http.Error(w, "invalid YAML: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := appconfig.Save(configPath, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<p>saved. restart server to apply changes.</p><a href=\"/\">back</a>"))
	})

	server := &http.Server{Addr: addr, Handler: util.LogMiddleware(mux, logWriter)}
	s := &Server{srv: server}
	errCh := make(chan error, 1)
	go func() {
		util.Logf(logWriter, "[webui] listening on %s\n", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	return s, errCh
}

func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func mustYAML(cfg appconfig.File) string {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

type pageData struct {
	ConfigYAML string
}

var pageTmpl = template.Must(template.New("webui").Parse(`<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>easytools Web UI</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; }
    textarea { width: 100%; height: 60vh; font-family: monospace; font-size: 0.9rem; }
    form { max-width: 960px; }
    .notice { margin-bottom: 1rem; padding: 1rem; background: #eef; border-radius: 8px; }
  </style>
</head>
<body>
  <h1>easytools Web 設定</h1>
  <div class="notice">
    <p>YAML を編集して保存すると <code>tools.yaml</code> に反映されます。サーバーは再起動するまで新しい設定を読み込みません。</p>
  </div>
  <form method="post" action="/save">
    <textarea name="content">{{.ConfigYAML}}</textarea>
    <p><button type="submit">保存</button></p>
  </form>
</body>
</html>`))
