package execrunner

import (
	"context"
	"testing"

	"github.com/yashubustudio/easytools/internal/model"
)

func TestRunOnceSanitizesParams(t *testing.T) {
	cfg := &model.ServerConfig{Tools: map[string]model.Tool{
		"echo": {
			Cmd:  "printf",
			Args: []string{"%s", "{{msg}}"},
			Input: model.ToolInputSpec{
				Params: []model.ToolInputField{{Name: "msg", Required: true}},
			},
		},
	}}

	res, status, err := RunOnce(context.Background(), cfg, &model.RunRequest{
		Tool:   "echo",
		Params: map[string]any{"msg": "hello", "ignored": "value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("unexpected status: %d", status)
	}
	if res.Stdout != "hello" {
		t.Fatalf("stdout not rendered: %q", res.Stdout)
	}

	if _, status, err := RunOnce(context.Background(), cfg, &model.RunRequest{Tool: "echo"}); err == nil || status != 400 {
		t.Fatalf("expected validation error for missing param")
	}
}

func TestRunOnceEnvFiltering(t *testing.T) {
	cfg := &model.ServerConfig{Tools: map[string]model.Tool{
		"env": {
			Cmd:      "sh",
			Args:     []string{"-c", "printf %s \"$TOKEN\""},
			AllowEnv: []string{"TOKEN"},
			Input: model.ToolInputSpec{
				Env: []model.ToolInputField{{Name: "TOKEN", Required: true}},
			},
		},
	}}

	res, status, err := RunOnce(context.Background(), cfg, &model.RunRequest{
		Tool: "env",
		Env:  map[string]string{"TOKEN": "secret", "OTHER": "nope"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("unexpected status: %d", status)
	}
	if res.Stdout != "secret" {
		t.Fatalf("env not passed through: %q", res.Stdout)
	}

	if _, status, err := RunOnce(context.Background(), cfg, &model.RunRequest{
		Tool: "env",
	}); err == nil || status != 400 {
		t.Fatalf("expected error when required env missing")
	}
}

func TestRunOnceOutputMask(t *testing.T) {
	cfg := &model.ServerConfig{Tools: map[string]model.Tool{
		"mask": {
			Cmd:  "printf",
			Args: []string{"%s", "{{msg}}"},
			Input: model.ToolInputSpec{
				Params: []model.ToolInputField{{Name: "msg", Required: true}},
			},
			Output: model.ToolOutputSpec{
				Fields: []model.ToolOutputField{{Name: "stdout", Mask: true}},
			},
		},
	}}

	res, status, err := RunOnce(context.Background(), cfg, &model.RunRequest{
		Tool:   "mask",
		Params: map[string]any{"msg": "private"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("unexpected status: %d", status)
	}
	if res.Stdout != "***" {
		t.Fatalf("stdout should be masked, got %q", res.Stdout)
	}
}
