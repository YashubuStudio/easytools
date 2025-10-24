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

	res, status, err = RunOnce(context.Background(), cfg, &model.RunRequest{
		Tool: "env",
	})
	if err != nil {
		t.Fatalf("unexpected error when env missing: %v", err)
	}
	if status != 200 {
		t.Fatalf("unexpected status when env missing: %d", status)
	}
	if res.Stdout != "" {
		t.Fatalf("expected empty stdout when env missing, got %q", res.Stdout)
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

func TestRecordMissingEnvAddsSpecs(t *testing.T) {
	cfg := &model.ServerConfig{Tools: map[string]model.Tool{
		"tool": {
			Cmd: "echo",
		},
	}}

	added := cfg.RecordMissingEnv("tool", []string{"TOKEN", "TOKEN", "API_KEY"})
	if len(added) != 2 {
		t.Fatalf("expected 2 added env fields, got %d", len(added))
	}

	tool := cfg.Tools["tool"]
	if !containsString(tool.AllowEnv, "API_KEY") || !containsString(tool.AllowEnv, "TOKEN") {
		t.Fatalf("allow list missing expected entries: %#v", tool.AllowEnv)
	}

	names := make([]string, 0, len(tool.Input.Env))
	for _, field := range tool.Input.Env {
		names = append(names, field.Name)
	}
	if !containsString(names, "TOKEN") || !containsString(names, "API_KEY") {
		t.Fatalf("input env missing expected entries: %#v", names)
	}
}

func containsString(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
