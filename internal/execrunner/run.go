package execrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/yashubustudio/easytools/internal/model"
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
	args, err := RenderArgs(tool.Args, req.Params)
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
	if len(req.Env) > 0 && len(tool.AllowEnv) > 0 {
		allowed := map[string]struct{}{}
		for _, k := range tool.AllowEnv {
			allowed[k] = struct{}{}
		}
		for k, v := range req.Env {
			if _, ok := allowed[k]; ok {
				env = append(env, k+"="+v)
			}
		}
	}
	cmd.Env = append(env, defaultSafeEnv()...)
	if req.Stdin != "" {
		if !tool.AllowStdin {
			return &model.RunResponse{Tool: req.Tool}, 400, errors.New("stdin not allowed for this tool")
		}
		cmd.Stdin = strings.NewReader(req.Stdin)
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
	return resp, status, nil
}
