package appconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yashubustudio/easytools/internal/model"
)

const DefaultPath = "tools.yaml"

type File struct {
	Addr       string                `yaml:"addr"`
	BasePath   string                `yaml:"base_path"`
	Paths      model.Paths           `yaml:"paths"`
	APIKey     string                `yaml:"api_key"`
	CORS       *bool                 `yaml:"cors"`
	CORSOrigin string                `yaml:"cors_origin"`
	ListenAll  *bool                 `yaml:"listen_all"`
	Tools      map[string]model.Tool `yaml:"tools"`
}

func Default() File {
	cors := true
	listenAll := false
	return File{
		Addr:     ":8080",
		BasePath: "/v1",
		Paths: model.Paths{
			Run:        "/run",
			Tools:      "/tools",
			Reload:     "/reload",
			Health:     "/healthz",
			MCPPackage: "/mcp/package",
			MCPInvoke:  "/mcp/run",
		},
		CORS:      &cors,
		ListenAll: &listenAll,
		APIKey:    "devkey",
		Tools: map[string]model.Tool{
			"echo": {
				Cmd:       "echo",
				Args:      []string{"{{msg}}"},
				Timeout:   "5s",
				MaxStdout: 1 << 20,
			},
		},
	}
}

func Load(path string) (File, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Tools == nil || len(cfg.Tools) == 0 {
		cfg.Tools = Default().Tools
	}
	if cfg.CORS == nil {
		defaultVal := true
		cfg.CORS = &defaultVal
	}
	if cfg.ListenAll == nil {
		defaultVal := false
		cfg.ListenAll = &defaultVal
	}
	cfg.Paths = applyDefaultPaths(cfg.Paths)
	return cfg, nil
}

func Save(path string, cfg File) error {
	cfg.Paths = applyDefaultPaths(cfg.Paths)
	if cfg.Tools == nil {
		cfg.Tools = map[string]model.Tool{}
	}
	if cfg.CORS == nil {
		defaultVal := true
		cfg.CORS = &defaultVal
	}
	if cfg.ListenAll == nil {
		defaultVal := false
		cfg.ListenAll = &defaultVal
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func applyDefaultPaths(p model.Paths) model.Paths {
	def := Default().Paths
	if strings.TrimSpace(p.Run) == "" {
		p.Run = def.Run
	}
	if strings.TrimSpace(p.Tools) == "" {
		p.Tools = def.Tools
	}
	if strings.TrimSpace(p.Reload) == "" {
		p.Reload = def.Reload
	}
	if strings.TrimSpace(p.Health) == "" {
		p.Health = def.Health
	}
	if strings.TrimSpace(p.MCPPackage) == "" {
		p.MCPPackage = def.MCPPackage
	}
	if strings.TrimSpace(p.MCPInvoke) == "" {
		p.MCPInvoke = def.MCPInvoke
	}
	return p
}

func (f File) ToServerConfig() *model.ServerConfig {
	cfg := &model.ServerConfig{
		Addr:       f.effectiveAddr(),
		BasePath:   strings.TrimSpace(f.BasePath),
		APIKey:     f.APIKey,
		CORS:       f.corsValue(),
		CORSOrigin: strings.TrimSpace(f.CORSOrigin),
		Tools:      f.Tools,
		Paths:      applyDefaultPaths(f.Paths),
	}
	return cfg
}

func (f File) effectiveAddr() string {
	addr := strings.TrimSpace(f.Addr)
	if addr == "" {
		addr = Default().Addr
	}
	if strings.Contains(addr, ":") && !strings.HasPrefix(addr, ":") {
		return addr
	}
	host := "localhost"
	if f.listenAllValue() {
		host = "0.0.0.0"
	}
	if strings.HasPrefix(addr, ":") {
		return host + addr
	}
	return host + ":" + addr
}

func (f File) listenAllValue() bool {
	if f.ListenAll == nil {
		return *Default().ListenAll
	}
	return *f.ListenAll
}

func (f File) corsValue() bool {
	if f.CORS == nil {
		return *Default().CORS
	}
	return *f.CORS
}

func (f *File) UpdateFromServerConfig(cfg *model.ServerConfig) {
	if cfg == nil {
		return
	}
	f.Addr = cfg.Addr
	f.BasePath = cfg.BasePath
	f.APIKey = cfg.APIKey
	f.Paths = cfg.Paths
	cors := cfg.CORS
	f.CORS = &cors
	f.CORSOrigin = cfg.CORSOrigin
}

func (f *File) SetListenAll(v bool) {
	f.ListenAll = &v
}

func (f *File) SetCORS(v bool) {
	f.CORS = &v
}

func (f *File) ListenAllOrDefault() bool { return f.listenAllValue() }

func (f *File) CORSOrDefault() bool { return f.corsValue() }
