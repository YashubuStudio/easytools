package mcp

import (
	"testing"

	"github.com/yashubustudio/easytools/internal/model"
)

func TestBuildPackage(t *testing.T) {
	cfg := &model.ServerConfig{
		BasePath: "/api",
		Paths: model.Paths{
			MCPPackage: "/mcp/manifest",
			MCPInvoke:  "/mcp/exec",
		},
		Tools: map[string]model.Tool{
			"echo": {
				Group:      "demo",
				Cmd:        "echo",
				Args:       []string{"{{msg}}", "--times", "{{count}}"},
				WorkDir:    "/tmp",
				Env:        map[string]string{"STATIC": "1"},
				AllowEnv:   []string{"TOKEN", "DEBUG"},
				AllowStdin: true,
				Timeout:    "5s",
				MaxStdout:  123,
				MaxStderr:  456,
				Input: model.ToolInputSpec{
					Params: []model.ToolInputField{
						{Name: "msg", Description: "Message to echo", Required: true},
						{Name: "count", Description: "Repeat count"},
					},
					Env: []model.ToolInputField{
						{Name: "TOKEN", Description: "Bearer token", Required: true},
					},
					Stdin: &model.ToolInputStdin{Description: "Optional extra payload"},
				},
				Output: model.ToolOutputSpec{
					Fields: []model.ToolOutputField{
						{Name: "stdout", Mask: true},
					},
				},
				MCP: &model.ToolMCPDescriptor{
					Name:      "echo-tool",
					Arguments: []string{"msg", "count"},
					Promise:   "Echo the provided message",
					RequestExample: map[string]any{
						"params": map[string]any{"msg": "hello"},
					},
					ResponseExample: map[string]any{
						"stdout": "***",
					},
					Description: "Outputs the message received via params or stdin.",
				},
			},
		},
	}

	pkg := BuildPackage(cfg)
	if pkg.BasePath != "/api" {
		t.Fatalf("unexpected base path: %s", pkg.BasePath)
	}
	if pkg.PackagePath != "/api/mcp/manifest" {
		t.Fatalf("unexpected package path: %s", pkg.PackagePath)
	}
	if pkg.InvokePath != "/api/mcp/exec" {
		t.Fatalf("unexpected invoke path: %s", pkg.InvokePath)
	}
	if len(pkg.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(pkg.Tools))
	}

	tool := pkg.Tools[0]
	if tool.Name != "echo" {
		t.Fatalf("unexpected tool name: %s", tool.Name)
	}
	if tool.Group != "demo" {
		t.Fatalf("unexpected tool group: %s", tool.Group)
	}
	if tool.Command != "echo" {
		t.Fatalf("unexpected command: %s", tool.Command)
	}
	if len(tool.Args) != 3 || tool.Args[0] != "{{msg}}" {
		t.Fatalf("unexpected args: %#v", tool.Args)
	}
	if tool.WorkDir != "/tmp" {
		t.Fatalf("unexpected workdir: %s", tool.WorkDir)
	}
	if tool.FixedEnv["STATIC"] != "1" {
		t.Fatalf("fixed env not copied: %#v", tool.FixedEnv)
	}
	if len(tool.AllowEnv) != 2 || tool.AllowEnv[0] != "DEBUG" || tool.AllowEnv[1] != "TOKEN" {
		t.Fatalf("allow env not sorted/captured: %#v", tool.AllowEnv)
	}
	if !tool.AllowStdin {
		t.Fatalf("allow stdin expected")
	}
	if tool.Timeout != "5s" {
		t.Fatalf("timeout lost: %s", tool.Timeout)
	}
	if tool.MaxStdout != 123 || tool.MaxStderr != 456 {
		t.Fatalf("limits not copied: %d %d", tool.MaxStdout, tool.MaxStderr)
	}

	paramsSchema, ok := tool.InputSchema.Properties["params"]
	if !ok {
		t.Fatalf("params schema missing: %#v", tool.InputSchema.Properties)
	}
	if paramsSchema == nil || len(paramsSchema.Required) != 1 || paramsSchema.Required[0] != "msg" {
		t.Fatalf("params schema required unexpected: %#v", paramsSchema)
	}
	stdinSchema := tool.InputSchema.Properties["stdin"]
	if stdinSchema == nil || stdinSchema.Description == "" {
		t.Fatalf("stdin schema missing")
	}
	envSchema := tool.InputSchema.Properties["env"]
	if envSchema == nil || len(envSchema.Required) != 1 || envSchema.Required[0] != "TOKEN" {
		t.Fatalf("env schema unexpected: %#v", envSchema)
	}
	if pkg.ResponseSchema.Properties["output"] == nil {
		t.Fatalf("package response schema missing output property")
	}
	if tool.ResponseSchema.Properties["stdout"] == nil {
		t.Fatalf("tool response schema missing stdout")
	}
	if tool.Descriptor == nil || tool.Descriptor.Name != "echo-tool" {
		t.Fatalf("descriptor missing or incorrect: %#v", tool.Descriptor)
	}
	if len(tool.Descriptor.Arguments) != 2 {
		t.Fatalf("descriptor arguments missing: %#v", tool.Descriptor.Arguments)
	}
}

func TestBuildPackageDefaults(t *testing.T) {
	pkg := BuildPackage(nil)
	if pkg.BasePath != "/v1" {
		t.Fatalf("expected default base path, got %s", pkg.BasePath)
	}
	if pkg.PackagePath != "/v1/mcp/package" {
		t.Fatalf("expected default package path, got %s", pkg.PackagePath)
	}
	if pkg.InvokePath != "/v1/mcp/run" {
		t.Fatalf("expected default invoke path, got %s", pkg.InvokePath)
	}
}

func TestInvokeRequestToRunRequest(t *testing.T) {
	req := &InvokeRequest{
		Name: "echo",
		Input: &InvokeInput{
			Params: map[string]any{"msg": "hello"},
			Env:    map[string]string{"TOKEN": "abc"},
			Stdin:  "payload",
		},
	}
	runReq, err := req.ToRunRequest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runReq.Tool != "echo" {
		t.Fatalf("tool mismatch: %s", runReq.Tool)
	}
	if runReq.Params["msg"].(string) != "hello" {
		t.Fatalf("params mismatch: %#v", runReq.Params)
	}
	if runReq.Env["TOKEN"] != "abc" {
		t.Fatalf("env mismatch: %#v", runReq.Env)
	}
	if runReq.Stdin != "payload" {
		t.Fatalf("stdin mismatch: %s", runReq.Stdin)
	}

	// mutate source maps to ensure deep copy
	req.Input.Params["msg"] = "bye"
	req.Input.Env["TOKEN"] = "zzz"
	if runReq.Params["msg"].(string) != "hello" {
		t.Fatalf("params should be copied, got %#v", runReq.Params)
	}
	if runReq.Env["TOKEN"] != "abc" {
		t.Fatalf("env should be copied, got %#v", runReq.Env)
	}

	if _, err := (&InvokeRequest{}).ToRunRequest(); err == nil {
		t.Fatalf("expected error for missing name")
	}
}

func TestBuildResponse(t *testing.T) {
	res := &model.RunResponse{
		Tool:     "echo",
		Command:  []string{"/bin/echo", "hello"},
		ExitCode: 0,
		Stdout:   "hello\n",
		Stderr:   "",
	}
	out := BuildResponse(res)
	if !out.Success {
		t.Fatalf("expected success")
	}
	if out.Name != "echo" {
		t.Fatalf("name mismatch: %s", out.Name)
	}
	if len(out.Output.Command) != 2 || out.Output.Command[0] != "/bin/echo" {
		t.Fatalf("command mismatch: %#v", out.Output.Command)
	}

	res.ExitCode = 2
	res.TimedOut = true
	out = BuildResponse(res)
	if out.Success {
		t.Fatalf("success should be false when exit code != 0 or timed out")
	}
}
