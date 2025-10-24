package execrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/yashubustudio/easytools/internal/model"
	"github.com/yashubustudio/easytools/internal/util"
)

type cappedBuffer struct {
	cap int
	b   []byte
}

func newCappedBuffer(cap int) *cappedBuffer {
	return &cappedBuffer{cap: cap, b: make([]byte, 0, min(cap, 64<<10))}
}
func (c *cappedBuffer) Write(p []byte) (int, error) {
	if len(c.b) >= c.cap {
		return len(p), nil
	}
	rem := c.cap - len(c.b)
	if len(p) > rem {
		p = p[:rem]
	}
	c.b = append(c.b, p...)
	return len(p), nil
}
func (c *cappedBuffer) String() string { return string(c.b) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func defaultSafeEnv() []string {
	vars := []string{}
	if p := os.Getenv("PATH"); p != "" {
		vars = append(vars, "PATH="+p)
	}
	if h := os.Getenv("HOME"); h != "" {
		vars = append(vars, "HOME="+h)
	}
	if runtime.GOOS == "windows" {
		if c := os.Getenv("COMSPEC"); c != "" {
			vars = append(vars, "COMSPEC="+c)
		}
	} else {
		if sh := os.Getenv("SHELL"); sh != "" {
			vars = append(vars, "SHELL="+sh)
		}
	}
	return vars
}

func RenderArgs(templates []string, params map[string]any) ([]string, error) {
	if params == nil {
		params = map[string]any{}
	}
	out := make([]string, 0, len(templates))
	for _, t := range templates {
		arg := t
		for k, v := range params {
			ph := "{{" + k + "}}"
			arg = strings.ReplaceAll(arg, ph, fmt.Sprint(v))
		}
		if strings.Contains(arg, "{{") {
			return nil, fmt.Errorf("unresolved template token in arg: %q", arg)
		}
		out = append(out, arg)
	}
	return out, nil
}

func RunOnce(ctx context.Context, cfg *model.ServerConfig, req *model.RunRequest) (*model.RunResponse, int, error) {
	tool, ok := cfg.Tools[req.Tool]
	if !ok {
		return &model.RunResponse{Tool: req.Tool}, 404, fmt.Errorf("unknown tool: %s", req.Tool)
	}
	params, err := sanitizeParams(tool, req.Params)
	if err != nil {
		return &model.RunResponse{Tool: req.Tool}, 400, err
	}
	envVars, missingEnv, err := sanitizeEnv(tool, req.Env)
	if err != nil {
		return &model.RunResponse{Tool: req.Tool}, 400, err
	}
	if len(missingEnv) > 0 {
		for _, name := range missingEnv {
			util.Logf(cfg.LogWriter, "[run] tool=%s env %s が足りません\n", req.Tool, name)
		}
		if added := cfg.RecordMissingEnv(req.Tool, missingEnv); len(added) > 0 {
			util.Logf(cfg.LogWriter, "[run] tool=%s Env項目を追加: %s\n", req.Tool, strings.Join(added, ", "))
		}
	}
	stdin, err := sanitizeStdin(tool, req.Stdin)
	if err != nil {
		return &model.RunResponse{Tool: req.Tool}, 400, err
	}

	args, err := RenderArgs(tool.Args, params)
	if err != nil {
		return &model.RunResponse{Tool: req.Tool}, 400, fmt.Errorf("arg template error: %w", err)
	}
	cmdPath, err := exec.LookPath(tool.Cmd)
	if err != nil {
		return &model.RunResponse{Tool: req.Tool}, 400, fmt.Errorf("cmd not found: %s", tool.Cmd)
	}
	timeout := 60 * time.Second
	if tool.Timeout != "" {
		if d, e := time.ParseDuration(tool.Timeout); e == nil {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	if tool.WorkDir != "" {
		cmd.Dir = tool.WorkDir
	}

	env := []string{}
	for k, v := range tool.Env {
		env = append(env, k+"="+v)
	}
	if len(envVars) > 0 {
		keys := make([]string, 0, len(envVars))
		for k := range envVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			env = append(env, k+"="+envVars[k])
		}
	}
	cmd.Env = append(env, defaultSafeEnv()...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	maxOut := tool.MaxStdout
	if maxOut == 0 {
		maxOut = 10 << 20
	}
	maxErr := tool.MaxStderr
	if maxErr == 0 {
		maxErr = 2 << 20
	}
	stdout := newCappedBuffer(maxOut)
	stderr := newCappedBuffer(maxErr)
	cmd.Stdout, cmd.Stderr = stdout, stderr

	start := time.Now()
	exit := 0
	err = cmd.Run()
	ended := time.Now()
	if err != nil {
		if ee := new(exec.ExitError); errors.As(err, &ee) {
			exit = ee.ExitCode()
		} else if errors.Is(err, context.DeadlineExceeded) {
			exit = 124
		} else {
			exit = 1
		}
	}
	resp := &model.RunResponse{
		Tool: req.Tool, Command: append([]string{cmdPath}, args...), ExitCode: exit,
		Stdout: stdout.String(), Stderr: stderr.String(), Duration: ended.Sub(start) / time.Millisecond,
		TimedOut: ctx.Err() == context.DeadlineExceeded, StartedAt: start, EndedAt: ended,
	}
	status := 200
	if exit != 0 {
		status = 400
	}
	applyOutputMask(tool, resp)
	return resp, status, nil
}

func sanitizeParams(tool model.Tool, params map[string]any) (map[string]any, error) {
	allowed := map[string]model.ToolInputField{}
	for _, field := range tool.Input.Params {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		allowed[name] = field
	}

	if len(allowed) == 0 {
		tokens := extractTemplateTokens(tool.Args)
		if len(tokens) == 0 {
			return nil, nil
		}
		sanitized := make(map[string]any, len(tokens))
		for _, token := range tokens {
			val, ok := params[token]
			if !ok {
				return nil, fmt.Errorf("missing required param: %s", token)
			}
			sanitized[token] = val
		}
		return sanitized, nil
	}

	var sanitized map[string]any
	for name, field := range allowed {
		if params != nil {
			if val, ok := params[name]; ok {
				if sanitized == nil {
					sanitized = map[string]any{}
				}
				sanitized[name] = val
				continue
			}
		}
		if field.Required {
			return nil, fmt.Errorf("missing required param: %s", name)
		}
	}
	if len(sanitized) == 0 {
		return nil, nil
	}
	return sanitized, nil
}

func sanitizeEnv(tool model.Tool, env map[string]string) (map[string]string, []string, error) {
	allowList := map[string]struct{}{}
	for _, name := range tool.AllowEnv {
		allowList[name] = struct{}{}
	}

	spec := map[string]model.ToolInputField{}
	for _, field := range tool.Input.Env {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		if len(allowList) > 0 {
			if _, ok := allowList[name]; !ok {
				return nil, nil, fmt.Errorf("env field %s not allowed by allow_env", name)
			}
		}
		spec[name] = field
	}

	if len(spec) == 0 && len(allowList) == 0 {
		return nil, nil, nil
	}

	var sanitized map[string]string
	missing := []string{}
	if len(spec) > 0 {
		for name, field := range spec {
			if env != nil {
				if val, ok := env[name]; ok {
					if sanitized == nil {
						sanitized = map[string]string{}
					}
					sanitized[name] = val
					continue
				}
			}
			if field.Required {
				missing = append(missing, name)
			}
		}
	} else {
		for name := range allowList {
			if env == nil {
				continue
			}
			if val, ok := env[name]; ok {
				if sanitized == nil {
					sanitized = map[string]string{}
				}
				sanitized[name] = val
			}
		}
	}

	if len(sanitized) == 0 {
		return nil, missing, nil
	}
	return sanitized, missing, nil
}

func sanitizeStdin(tool model.Tool, stdin string) (string, error) {
	if stdin == "" {
		if tool.Input.Stdin != nil && tool.Input.Stdin.Required {
			if !tool.AllowStdin {
				return "", fmt.Errorf("stdin required but not allowed")
			}
			return "", fmt.Errorf("missing required stdin")
		}
		return "", nil
	}
	if !tool.AllowStdin {
		return "", errors.New("stdin not allowed for this tool")
	}
	if tool.Input.Stdin != nil && tool.Input.Stdin.Required && strings.TrimSpace(stdin) == "" {
		return "", fmt.Errorf("missing required stdin")
	}
	return stdin, nil
}

func applyOutputMask(tool model.Tool, res *model.RunResponse) {
	if res == nil {
		return
	}
	if len(tool.Output.Fields) == 0 {
		return
	}
	for _, field := range tool.Output.Fields {
		name := strings.TrimSpace(strings.ToLower(field.Name))
		if name == "" {
			continue
		}
		replacement := field.Replacement
		if replacement == "" {
			replacement = "***"
		}
		switch name {
		case "stdout":
			res.Stdout = maskString(res.Stdout, field.Pattern, field.Mask, replacement)
		case "stderr":
			res.Stderr = maskString(res.Stderr, field.Pattern, field.Mask, replacement)
		case "command":
			if len(res.Command) == 0 {
				continue
			}
			masked := make([]string, len(res.Command))
			for i, part := range res.Command {
				masked[i] = maskString(part, field.Pattern, field.Mask, replacement)
			}
			res.Command = masked
		}
	}
}

func maskString(value, pattern string, mask bool, replacement string) string {
	if value == "" {
		return value
	}
	if pattern != "" {
		return strings.ReplaceAll(value, pattern, replacement)
	}
	if mask {
		return replacement
	}
	return value
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
