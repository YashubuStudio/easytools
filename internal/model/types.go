package model

import (
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
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
	LogWriter  io.Writer       `yaml:"-" json:"-"`

	mu sync.Mutex `yaml:"-" json:"-"`
}

// Convenience for handlers
func (c *ServerConfig) Validate() error { return nil }

// RecordMissingEnv registers the provided environment variable names as
// user-supplied requirements for the given tool. The method ensures that the
// tool's allow list and input specification include the variables so that GUI
// や API クライアントで項目が提示され、値を入力すればすぐ使えるようになります。
// 戻り値は新規に追加された項目名です。
func (c *ServerConfig) RecordMissingEnv(toolName string, envNames []string) []string {
	if c == nil || len(envNames) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	tool, ok := c.Tools[toolName]
	if !ok {
		return nil
	}

	allowSet := map[string]struct{}{}
	for _, name := range tool.AllowEnv {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		allowSet[trimmed] = struct{}{}
	}

	fieldSet := map[string]struct{}{}
	for _, field := range tool.Input.Env {
		trimmed := strings.TrimSpace(field.Name)
		if trimmed == "" {
			continue
		}
		fieldSet[trimmed] = struct{}{}
	}

	added := make([]string, 0, len(envNames))
	allowUpdated := false
	for _, raw := range envNames {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := allowSet[name]; !ok {
			tool.AllowEnv = append(tool.AllowEnv, name)
			allowSet[name] = struct{}{}
			allowUpdated = true
		}
		if _, ok := fieldSet[name]; ok {
			continue
		}
		tool.Input.Env = append(tool.Input.Env, ToolInputField{
			Name:        name,
			Description: "auto-added (missing during execution)",
		})
		fieldSet[name] = struct{}{}
		added = append(added, name)
	}

	if allowUpdated {
		sort.Strings(tool.AllowEnv)
	}
	if len(added) > 0 {
		sort.Slice(tool.Input.Env, func(i, j int) bool {
			return strings.TrimSpace(tool.Input.Env[i].Name) < strings.TrimSpace(tool.Input.Env[j].Name)
		})
	}

	if allowUpdated || len(added) > 0 {
		if c.Tools == nil {
			c.Tools = map[string]Tool{}
		}
		c.Tools[toolName] = tool
	}
	return added
}

// For status constants usage in other packages
const (
	StatusOK               = http.StatusOK
	StatusBadRequest       = http.StatusBadRequest
	StatusMethodNotAllowed = http.StatusMethodNotAllowed
	StatusUnauthorized     = http.StatusUnauthorized
	StatusNotFound         = http.StatusNotFound
)
