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

	mux := http.NewServeMux()
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if cfg.CORS {
				w.Header().Set("Access-Control-Allow-Origin", "*")
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

	mux.HandleFunc(util.JoinPathLike(cfg.BasePath, cfg.Paths.Tools), wrap(func(w http.ResponseWriter, r *http.Request) {
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
