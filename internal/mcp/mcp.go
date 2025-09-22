package mcp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yashubustudio/easytools/internal/model"
	"github.com/yashubustudio/easytools/internal/util"
)

const (
	defaultPackageName    = "easytools"
	defaultPackageVersion = "1.0"
	defaultPackagePath    = "/mcp/package"
	defaultInvokePath     = "/mcp/run"
)

// Package describes every registered tool as an MCP-compatible bundle.
type Package struct {
	Name           string     `json:"name"`
	Version        string     `json:"version"`
	BasePath       string     `json:"base_path"`
	PackagePath    string     `json:"package_path"`
	InvokePath     string     `json:"invoke_path"`
	ResponseSchema JSONSchema `json:"response_schema"`
	Tools          []Tool     `json:"tools"`
}

// Tool is the MCP view of a registered command.
type Tool struct {
	Name           string            `json:"name"`
	Group          string            `json:"group,omitempty"`
	Summary        string            `json:"summary"`
	Command        string            `json:"command"`
	Args           []string          `json:"args,omitempty"`
	WorkDir        string            `json:"workdir,omitempty"`
	FixedEnv       map[string]string `json:"fixed_env,omitempty"`
	AllowEnv       []string          `json:"allow_env,omitempty"`
	AllowStdin     bool              `json:"allow_stdin,omitempty"`
	Timeout        string            `json:"timeout,omitempty"`
	MaxStdout      int               `json:"max_stdout,omitempty"`
	MaxStderr      int               `json:"max_stderr,omitempty"`
	InputSchema    JSONSchema        `json:"input_schema"`
	ResponseSchema JSONSchema        `json:"response_schema"`
}

// JSONSchema is a simplified JSON schema representation used in manifests.
type JSONSchema struct {
	Type                 string                 `json:"type,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	Enum                 []string               `json:"enum,omitempty"`
}

// InvokeRequest is the MCP invocation shape that wraps the traditional RunRequest data.
type InvokeRequest struct {
	Name  string       `json:"name"`
	Input *InvokeInput `json:"input,omitempty"`
}

// InvokeInput contains the original EasyTools request fields in a single envelope.
type InvokeInput struct {
	Params map[string]any    `json:"params,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
	Stdin  string            `json:"stdin,omitempty"`
}

// Response is the single-output representation returned by the MCP endpoint.
type Response struct {
	Name    string         `json:"name"`
	Success bool           `json:"success"`
	Output  ResponseOutput `json:"output"`
}

// ResponseOutput mirrors execrunner.RunOnce results while remaining stable for MCP clients.
type ResponseOutput struct {
	Command    []string  `json:"command"`
	ExitCode   int       `json:"exit_code"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	DurationMs int64     `json:"duration_ms"`
	TimedOut   bool      `json:"timed_out"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
}

// BuildPackage converts the loaded server configuration into an MCP package manifest.
func BuildPackage(cfg *model.ServerConfig) Package {
	pkg := Package{
		Name:           defaultPackageName,
		Version:        defaultPackageVersion,
		BasePath:       normalizeBasePath(""),
		PackagePath:    util.JoinPathLike(normalizeBasePath(""), defaultPackagePath),
		InvokePath:     util.JoinPathLike(normalizeBasePath(""), defaultInvokePath),
		ResponseSchema: buildResponseSchema(),
	}
	if cfg == nil {
		return pkg
	}
	base := normalizeBasePath(cfg.BasePath)
	packagePath := cfg.Paths.MCPPackage
	if packagePath == "" {
		packagePath = defaultPackagePath
	}
	invokePath := cfg.Paths.MCPInvoke
	if invokePath == "" {
		invokePath = defaultInvokePath
	}
	pkg.BasePath = base
	pkg.PackagePath = util.JoinPathLike(base, packagePath)
	pkg.InvokePath = util.JoinPathLike(base, invokePath)
	names := make([]string, 0, len(cfg.Tools))
	for name := range cfg.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		tool := cfg.Tools[name]
		tools = append(tools, buildToolDefinition(name, tool))
	}
	pkg.Tools = tools
	return pkg
}

// BuildResponse wraps the legacy RunResponse into the MCP specific single-output payload.
func BuildResponse(res *model.RunResponse) Response {
	if res == nil {
		return Response{}
	}
	cmd := cloneStrings(res.Command)
	success := res.ExitCode == 0 && !res.TimedOut
	return Response{
		Name:    res.Tool,
		Success: success,
		Output: ResponseOutput{
			Command:    cmd,
			ExitCode:   res.ExitCode,
			Stdout:     res.Stdout,
			Stderr:     res.Stderr,
			DurationMs: int64(res.Duration),
			TimedOut:   res.TimedOut,
			StartedAt:  res.StartedAt,
			EndedAt:    res.EndedAt,
		},
	}
}

// ToRunRequest converts an InvokeRequest into the classic RunRequest structure.
func (r *InvokeRequest) ToRunRequest() (*model.RunRequest, error) {
	if r == nil {
		return nil, errors.New("nil request")
	}
	name := strings.TrimSpace(r.Name)
	if name == "" {
		return nil, errors.New("missing tool name")
	}
	runReq := &model.RunRequest{Tool: name}
	if r.Input != nil {
		if len(r.Input.Params) > 0 {
			runReq.Params = cloneParams(r.Input.Params)
		}
		if len(r.Input.Env) > 0 {
			runReq.Env = cloneEnv(r.Input.Env)
		}
		runReq.Stdin = r.Input.Stdin
	}
	return runReq, nil
}

func buildToolDefinition(name string, tool model.Tool) Tool {
	summary := buildSummary(tool)
	return Tool{
		Name:           name,
		Group:          tool.Group,
		Summary:        summary,
		Command:        tool.Cmd,
		Args:           cloneStrings(tool.Args),
		WorkDir:        tool.WorkDir,
		FixedEnv:       cloneEnv(tool.Env),
		AllowEnv:       cloneAndSortStrings(tool.AllowEnv),
		AllowStdin:     tool.AllowStdin,
		Timeout:        tool.Timeout,
		MaxStdout:      tool.MaxStdout,
		MaxStderr:      tool.MaxStderr,
		InputSchema:    buildInputSchema(tool),
		ResponseSchema: buildOutputSchema(),
	}
}

func buildSummary(tool model.Tool) string {
	parts := []string{}
	if tool.Cmd != "" {
		parts = append(parts, tool.Cmd)
	}
	if len(tool.Args) > 0 {
		parts = append(parts, strings.Join(tool.Args, " "))
	}
	if len(parts) == 0 {
		return "Execute external command"
	}
	return fmt.Sprintf("Execute %s", strings.Join(parts, " "))
}

func buildInputSchema(tool model.Tool) JSONSchema {
	props := map[string]*JSONSchema{}
	required := []string{}

	if schema, needsParam := buildParamsSchema(tool.Args); schema != nil {
		props["params"] = schema
		if needsParam {
			required = append(required, "params")
		}
	}
	if schema := buildEnvSchema(tool.AllowEnv); schema != nil {
		props["env"] = schema
	}
	if tool.AllowStdin {
		props["stdin"] = &JSONSchema{
			Type:        "string",
			Description: "Text passed to the command's standard input.",
		}
	}
	additionalFalse := boolPtr(false)
	if len(props) == 0 {
		return JSONSchema{
			Type:                 "object",
			Description:          "Input payload for the tool.",
			AdditionalProperties: additionalFalse,
		}
	}
	return JSONSchema{
		Type:                 "object",
		Description:          "Input payload for the tool.",
		Properties:           props,
		Required:             required,
		AdditionalProperties: additionalFalse,
	}
}

func buildParamsSchema(args []string) (*JSONSchema, bool) {
	tokens := extractTemplateTokens(args)
	schema := &JSONSchema{
		Type:        "object",
		Description: "Values replacing template tokens defined in args.",
	}
	additional := boolPtr(false)
	schema.AdditionalProperties = additional
	if len(tokens) == 0 {
		return schema, false
	}
	props := make(map[string]*JSONSchema, len(tokens))
	for _, token := range tokens {
		props[token] = &JSONSchema{Type: "string"}
	}
	schema.Properties = props
	schema.Required = cloneStrings(tokens)
	return schema, true
}

func buildEnvSchema(allowed []string) *JSONSchema {
	if len(allowed) == 0 {
		return nil
	}
	keys := cloneAndSortStrings(allowed)
	props := make(map[string]*JSONSchema, len(keys))
	for _, key := range keys {
		props[key] = &JSONSchema{Type: "string"}
	}
	return &JSONSchema{
		Type:                 "object",
		Description:          "Environment variables forwarded to the command.",
		Properties:           props,
		AdditionalProperties: boolPtr(false),
	}
}

func buildResponseSchema() JSONSchema {
	output := buildOutputSchema()
	props := map[string]*JSONSchema{
		"name":    {Type: "string", Description: "Tool name that produced the response."},
		"success": {Type: "boolean", Description: "True when the command exited with status 0 and no timeout."},
		"output":  &output,
	}
	return JSONSchema{
		Type:                 "object",
		Description:          "Response returned from the MCP execution endpoint.",
		Properties:           props,
		Required:             []string{"name", "success", "output"},
		AdditionalProperties: boolPtr(false),
	}
}

func buildOutputSchema() JSONSchema {
	commandItems := &JSONSchema{Type: "string"}
	props := map[string]*JSONSchema{
		"command":     {Type: "array", Items: commandItems, Description: "Executed command path followed by arguments."},
		"exit_code":   {Type: "integer", Description: "Process exit code."},
		"stdout":      {Type: "string", Description: "Captured standard output."},
		"stderr":      {Type: "string", Description: "Captured standard error."},
		"duration_ms": {Type: "integer", Description: "Execution duration reported in milliseconds."},
		"timed_out":   {Type: "boolean", Description: "Whether the process hit the configured timeout."},
		"started_at":  {Type: "string", Description: "Start timestamp in RFC3339."},
		"ended_at":    {Type: "string", Description: "End timestamp in RFC3339."},
	}
	return JSONSchema{
		Type:                 "object",
		Description:          "Command execution details.",
		Properties:           props,
		Required:             []string{"command", "exit_code", "stdout", "stderr", "duration_ms", "timed_out", "started_at", "ended_at"},
		AdditionalProperties: boolPtr(false),
	}
}

func normalizeBasePath(base string) string {
	if base == "" {
		base = "/v1"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	if len(base) > 1 {
		base = strings.TrimRight(base, "/")
	}
	return base
}

func extractTemplateTokens(args []string) []string {
	seen := map[string]struct{}{}
	for _, arg := range args {
		start := 0
		for start < len(arg) {
			open := strings.Index(arg[start:], "{{")
			if open == -1 {
				break
			}
			open += start
			close := strings.Index(arg[open+2:], "}}")
			if close == -1 {
				break
			}
			close += open + 2
			token := arg[open+2 : close]
			if token != "" {
				seen[token] = struct{}{}
			}
			start = close + 2
		}
	}
	tokens := make([]string, 0, len(seen))
	for token := range seen {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneAndSortStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

func cloneEnv(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneParams(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func boolPtr(b bool) *bool {
	return &b
}
