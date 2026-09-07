package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestToolsCallLoadArgs(t *testing.T) {
	t.Run("inline JSON object", func(t *testing.T) {
		c := &ToolsCallCmd{Args: `{"id":"abc","n":2}`}
		got, err := c.loadArgs(strings.NewReader(""))
		if err != nil {
			t.Fatalf("loadArgs() error = %v", err)
		}
		if got["id"] != "abc" {
			t.Fatalf("id = %v, want abc", got["id"])
		}
		if got["n"].(float64) != 2 {
			t.Fatalf("n = %v, want 2", got["n"])
		}
	})

	t.Run("empty defaults to empty object", func(t *testing.T) {
		c := &ToolsCallCmd{Args: "  "}
		got, err := c.loadArgs(strings.NewReader(""))
		if err != nil {
			t.Fatalf("loadArgs() error = %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})

	t.Run("reads from stdin when args-file is '-'", func(t *testing.T) {
		c := &ToolsCallCmd{ArgsFile: "-"}
		got, err := c.loadArgs(strings.NewReader(`{"from":"stdin"}`))
		if err != nil {
			t.Fatalf("loadArgs() error = %v", err)
		}
		if got["from"] != "stdin" {
			t.Fatalf("from = %v, want stdin", got["from"])
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		c := &ToolsCallCmd{Args: "not json"}
		if _, err := c.loadArgs(strings.NewReader("")); err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})
}

func TestNewToolSummaries(t *testing.T) {
	tools := []mcp.Tool{{Name: "a", Description: "desc-a"}}
	tools[0].InputSchema.Type = "object"

	withSchema := newToolSummaries(tools, true)
	if withSchema[0].InputSchema == nil {
		t.Fatal("expected inputSchema to be carried when includeSchema=true")
	}

	withoutSchema := newToolSummaries(tools, false)
	if withoutSchema[0].InputSchema != nil {
		t.Fatal("expected inputSchema to be omitted when includeSchema=false")
	}
}

func TestRenderToolResult(t *testing.T) {
	textResult := &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent("hello world")},
	}
	errResult := &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{mcp.NewTextContent("boom")},
	}

	t.Run("text mode writes the content", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderToolResult(&buf, "t", textResult, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(buf.String()) != "hello world" {
			t.Fatalf("output = %q, want %q", buf.String(), "hello world")
		}
	})

	t.Run("raw mode writes JSON", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderToolResult(&buf, "t", textResult, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), `"hello world"`) {
			t.Fatalf("raw output missing content: %q", buf.String())
		}
	})

	t.Run("tool error returns an error and writes no text", func(t *testing.T) {
		var buf bytes.Buffer
		err := renderToolResult(&buf, "mytool", errResult, false)
		if err == nil {
			t.Fatal("expected an error for IsError result")
		}
		if !strings.Contains(err.Error(), "mytool") || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("error = %q, want tool name + message", err.Error())
		}
		if buf.Len() != 0 {
			t.Fatalf("expected no stdout for tool error, got %q", buf.String())
		}
	})
}

func TestToolResultText(t *testing.T) {
	if got := toolResultText(nil); got != "" {
		t.Fatalf("nil result = %q, want empty", got)
	}

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("hello "),
			mcp.NewTextContent("world"),
		},
	}
	if got := toolResultText(result); got != "hello world" {
		t.Fatalf("toolResultText() = %q, want %q", got, "hello world")
	}
}
