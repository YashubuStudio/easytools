package model

import (
	"net/http"
	"time"
)

type Tool struct {
	Group      string            `yaml:"group" json:"group"`
	Cmd        string            `yaml:"cmd" json:"cmd"`
	Args       []string          `yaml:"args" json:"args"`
	WorkDir    string            `yaml:"workdir" json:"workdir"`
	Env        map[string]string `yaml:"env" json:"env"`
	AllowEnv   []string          `yaml:"allow_env" json:"allow_env"`
	Timeout    string            `yaml:"timeout" json:"timeout"`
	MaxStdout  int               `yaml:"max_stdout" json:"max_stdout"`
	MaxStderr  int               `yaml:"max_stderr" json:"max_stderr"`
	AllowStdin bool              `yaml:"allow_stdin" json:"allow_stdin"`
}

type RunRequest struct {
	Tool   string            `json:"tool"`
	Params map[string]any    `json:"params"`
	Env    map[string]string `json:"env"`
	Stdin  string            `json:"stdin"`
}

type RunResponse struct {
	Tool      string        `json:"tool"`
	Command   []string      `json:"command"`
	ExitCode  int           `json:"exit_code"`
	Stdout    string        `json:"stdout"`
	Stderr    string        `json:"stderr"`
	Duration  time.Duration `json:"duration_ms"`
	TimedOut  bool          `json:"timed_out"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
}

type Paths struct {
	Run    string `yaml:"run"`
	Tools  string `yaml:"tools"`
	Reload string `yaml:"reload"`
	Health string `yaml:"health"`
}

type ServerConfig struct {
	Addr     string          `yaml:"addr"`
	BasePath string          `yaml:"base_path"`
	APIKey   string          `yaml:"api_key"`
	CORS     bool            `yaml:"cors"`
	Tools    map[string]Tool `yaml:"tools"`
	Paths    Paths           `yaml:"paths"`
}

// Convenience for handlers
func (c *ServerConfig) Validate() error { return nil }

// For status constants usage in other packages
const (
	StatusOK               = http.StatusOK
	StatusBadRequest       = http.StatusBadRequest
	StatusMethodNotAllowed = http.StatusMethodNotAllowed
	StatusUnauthorized     = http.StatusUnauthorized
	StatusNotFound         = http.StatusNotFound
)
