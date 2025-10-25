package server

import (
	"encoding/json"
	"testing"

	"github.com/yashubustudio/easytools/internal/mcp"
	"github.com/yashubustudio/easytools/internal/model"
)

func TestDecodeInvokeOrRun_MCPRequest(t *testing.T) {
	body, err := json.Marshal(mcp.InvokeRequest{
		Name: "echo",
		Input: &mcp.InvokeInput{
			Params: map[string]any{"text": "hi"},
		},
	})
	if err != nil {
		t.Fatalf("marshal mcp request: %v", err)
	}

	runReq, isMCP, err := decodeInvokeOrRun(body)
	if err != nil {
		t.Fatalf("decodeInvokeOrRun returned error: %v", err)
	}
	if !isMCP {
		t.Fatalf("expected MCP flag to be true")
	}
	if runReq.Tool != "echo" {
		t.Fatalf("unexpected tool: %s", runReq.Tool)
	}
	if got := runReq.Params["text"]; got != "hi" {
		t.Fatalf("unexpected param: %v", got)
	}
}

func TestDecodeInvokeOrRun_RunAlias(t *testing.T) {
	body, err := json.Marshal(model.RunRequest{
		Tool:   "echo",
		Params: map[string]any{"text": "hi"},
	})
	if err != nil {
		t.Fatalf("marshal run request: %v", err)
	}

	runReq, isMCP, err := decodeInvokeOrRun(body)
	if err != nil {
		t.Fatalf("decodeInvokeOrRun returned error: %v", err)
	}
	if isMCP {
		t.Fatalf("expected MCP flag to be false")
	}
	if runReq.Tool != "echo" {
		t.Fatalf("unexpected tool: %s", runReq.Tool)
	}
}

func TestDecodeInvokeOrRun_Invalid(t *testing.T) {
	_, _, err := decodeInvokeOrRun([]byte(`{"name":123}`))
	if err == nil {
		t.Fatalf("expected error for invalid payload")
	}
}
