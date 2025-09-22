package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yashubustudio/easytools/internal/execrunner"
	"github.com/yashubustudio/easytools/internal/mcp"
	"github.com/yashubustudio/easytools/internal/model"
	"github.com/yashubustudio/easytools/internal/util"
)

type LegacyServer struct {
	mu        sync.Mutex
	srv       *http.Server
	cfg       *model.ServerConfig
	LogWriter interface{ Write([]byte) (int, error) }
}

func (s *LegacyServer) Start(cfg *model.ServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return errors.New("server already running")
	}
	if cfg.Tools == nil || len(cfg.Tools) == 0 {
		return errors.New("no tools defined")
	}
	if cfg.BasePath == "" {
		cfg.BasePath = "/v1"
	}
	if !strings.HasPrefix(cfg.BasePath, "/") {
		cfg.BasePath = "/" + cfg.BasePath
	}
	if cfg.Paths.Run == "" {
		cfg.Paths.Run = "/run"
	}
	if cfg.Paths.Tools == "" {
		cfg.Paths.Tools = "/tools"
	}
	if cfg.Paths.Reload == "" {
		cfg.Paths.Reload = "/reload"
	}
	if cfg.Paths.Health == "" {
		cfg.Paths.Health = "/healthz"
	}
	if cfg.Paths.MCPPackage == "" {
		cfg.Paths.MCPPackage = "/mcp/package"
	}
	if cfg.Paths.MCPInvoke == "" {
		cfg.Paths.MCPInvoke = "/mcp/run"
	}

	mux := http.NewServeMux()
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if cfg.CORS {
				origin := "*"
				if cfg.CORSOrigin != "" {
					origin = cfg.CORSOrigin
				}
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			if cfg.APIKey != "" && r.Header.Get("X-API-Key") != cfg.APIKey {
				util.WriteJSON(w, model.StatusUnauthorized, map[string]any{"error": "missing/invalid api key"})
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc(util.JoinPathLike(cfg.BasePath, cfg.Paths.Health), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	toolsBase := util.JoinPathLike(cfg.BasePath, cfg.Paths.Tools)
	mux.HandleFunc(toolsBase, wrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			util.WriteJSON(w, model.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		names := make([]string, 0, len(cfg.Tools))
		for k := range cfg.Tools {
			names = append(names, k)
		}
		sort.Strings(names)
		util.WriteJSON(w, model.StatusOK, map[string]any{"tools": names})
	}))

	if cfg.APIKey == "" {
		mux.HandleFunc(toolsBase+"/", wrap(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				util.WriteJSON(w, model.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			p := strings.TrimPrefix(r.URL.Path, toolsBase)
			p = strings.TrimPrefix(p, "/")
			parts := strings.Split(p, "/")
			if len(parts) == 0 || parts[len(parts)-1] == "" || len(parts) > 2 {
				util.WriteJSON(w, model.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			name := parts[len(parts)-1]
			if _, ok := cfg.Tools[name]; !ok {
				util.WriteJSON(w, model.StatusNotFound, map[string]any{"error": "tool not found"})
				return
			}
			res, status, _ := execrunner.RunOnce(r.Context(), cfg, &model.RunRequest{Tool: name})
			util.WriteJSON(w, status, res)
		}))
	}

	mux.HandleFunc(util.JoinPathLike(cfg.BasePath, cfg.Paths.Reload), wrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			util.WriteJSON(w, model.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		util.WriteJSON(w, model.StatusOK, map[string]any{"ok": true})
	}))

	mux.HandleFunc(util.JoinPathLike(cfg.BasePath, cfg.Paths.Run), wrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			util.WriteJSON(w, model.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		var req model.RunRequest
		if err := jsonNewDecoderMax(r, 2<<20).Decode(&req); err != nil {
			util.WriteJSON(w, model.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		res, status, _ := execrunner.RunOnce(r.Context(), cfg, &req)
		util.WriteJSON(w, status, res)
	}))

	mux.HandleFunc(util.JoinPathLike(cfg.BasePath, cfg.Paths.MCPPackage), wrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			util.WriteJSON(w, model.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		pkg := mcp.BuildPackage(cfg)
		util.WriteJSON(w, model.StatusOK, pkg)
	}))

	mux.HandleFunc(util.JoinPathLike(cfg.BasePath, cfg.Paths.MCPInvoke), wrap(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			util.WriteJSON(w, model.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		var invokeReq mcp.InvokeRequest
		if err := jsonNewDecoderMax(r, 2<<20).Decode(&invokeReq); err != nil {
			util.WriteJSON(w, model.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		runReq, err := invokeReq.ToRunRequest()
		if err != nil {
			util.WriteJSON(w, model.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		res, status, _ := execrunner.RunOnce(r.Context(), cfg, runReq)
		util.WriteJSON(w, status, mcp.BuildResponse(res))
	}))

	s.cfg = cfg
	s.srv = &http.Server{Addr: cfg.Addr, Handler: util.LogMiddleware(mux, writerOrNil(s.LogWriter))}
	go func() {
		util.Logf(writerOrNil(s.LogWriter), "[srv] listening on %s (base=%s)\n", cfg.Addr, cfg.BasePath)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			util.Logf(writerOrNil(s.LogWriter), "[srv] error: %v\n", err)
		}
		util.Logf(writerOrNil(s.LogWriter), "[srv] server exited\n")
	}()
	return nil
}

func (s *LegacyServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := s.srv.Shutdown(ctx)
	s.srv = nil
	return err
}

// --- helpers ---

// tiny local helper for size-capped JSON decode
func jsonNewDecoderMax(r *http.Request, n int64) *jsonDecoder {
	return &jsonDecoder{r: http.MaxBytesReader(nil, r.Body, n)}
}

type jsonDecoder struct{ r io.Reader }

func (d *jsonDecoder) Decode(v any) error {
	return json.NewDecoder(d.r).Decode(v)
}

// avoid importing log here; keep it generic
func writerOrNil(w interface{ Write([]byte) (int, error) }) io.Writer {
	if w == nil {
		return nil
	}
	return w
}
