package model

import (
	"net/http"
	"time"
)

type Tool struct {
	Group      string             `yaml:"group" json:"group"`
	Cmd        string             `yaml:"cmd" json:"cmd"`
	Args       []string           `yaml:"args" json:"args"`
	WorkDir    string             `yaml:"workdir" json:"workdir"`
	Env        map[string]string  `yaml:"env" json:"env"`
	AllowEnv   []string           `yaml:"allow_env" json:"allow_env"`
	Timeout    string             `yaml:"timeout" json:"timeout"`
	MaxStdout  int                `yaml:"max_stdout" json:"max_stdout"`
	MaxStderr  int                `yaml:"max_stderr" json:"max_stderr"`
	AllowStdin bool               `yaml:"allow_stdin" json:"allow_stdin"`
	Input      ToolInputSpec      `yaml:"input" json:"input"`
	Output     ToolOutputSpec     `yaml:"output" json:"output"`
	MCP        *ToolMCPDescriptor `yaml:"mcp" json:"mcp"`
}

type ToolInputSpec struct {
	Params []ToolInputField `yaml:"params" json:"params"`
	Env    []ToolInputField `yaml:"env" json:"env"`
	Stdin  *ToolInputStdin  `yaml:"stdin" json:"stdin"`
}

type ToolInputField struct {
	Name        string `yaml:"name" json:"name"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type ToolInputStdin struct {
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type ToolOutputSpec struct {
	Fields []ToolOutputField `yaml:"fields" json:"fields"`
}

type ToolOutputField struct {
	Name        string `yaml:"name" json:"name"`
	Mask        bool   `yaml:"mask,omitempty" json:"mask,omitempty"`
	Pattern     string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Replacement string `yaml:"replacement,omitempty" json:"replacement,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type ToolMCPDescriptor struct {
	Name            string         `yaml:"name" json:"name"`
	Arguments       []string       `yaml:"arguments" json:"arguments"`
	Promise         string         `yaml:"promise,omitempty" json:"promise,omitempty"`
	RequestExample  map[string]any `yaml:"request_example,omitempty" json:"request_example,omitempty"`
	ResponseExample map[string]any `yaml:"response_example,omitempty" json:"response_example,omitempty"`
	Description     string         `yaml:"description,omitempty" json:"description,omitempty"`
	Notes           map[string]any `yaml:"notes,omitempty" json:"notes,omitempty"`
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
	Run        string `yaml:"run"`
	Tools      string `yaml:"tools"`
	Reload     string `yaml:"reload"`
	Health     string `yaml:"health"`
	MCPPackage string `yaml:"mcp_package"`
	MCPInvoke  string `yaml:"mcp_invoke"`
}

type ServerConfig struct {
	Addr       string          `yaml:"addr"`
	BasePath   string          `yaml:"base_path"`
	APIKey     string          `yaml:"api_key"`
	CORS       bool            `yaml:"cors"`
	CORSOrigin string          `yaml:"cors_origin"`
	Tools      map[string]Tool `yaml:"tools"`
	Paths      Paths           `yaml:"paths"`
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
