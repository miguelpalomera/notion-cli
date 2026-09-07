package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/miguelpalomera/notion-cli/internal/cli"
	"github.com/miguelpalomera/notion-cli/internal/output"
)

// ToolsCmd groups the MCP tool commands. `list` stays the default so
// `notion-cli tools` keeps listing tools; `call` invokes any tool generically.
type ToolsCmd struct {
	List ToolsListCmd `cmd:"" default:"1" help:"List available MCP tools"`
	Call ToolsCallCmd `cmd:"" help:"Call an MCP tool by name with JSON arguments"`
}

// ---------------------------------------------------------------------------
// tools list
// ---------------------------------------------------------------------------

type ToolsListCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

type toolSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

func (c *ToolsListCmd) Run(_ *Context) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	tools, err := client.ListTools(context.Background())
	if err != nil {
		output.PrintError(err)
		return err
	}

	return printTools(os.Stdout, newToolSummaries(tools, c.JSON), c.JSON)
}

// newToolSummaries projects MCP tools onto the CLI's summary DTO. The input
// schema is carried only for JSON output, where it is machine-useful.
func newToolSummaries(tools []mcp.Tool, includeSchema bool) []toolSummary {
	summaries := make([]toolSummary, len(tools))
	for i, t := range tools {
		summaries[i] = toolSummary{Name: t.Name, Description: t.Description}
		if includeSchema {
			summaries[i].InputSchema = t.InputSchema
		}
	}
	return summaries
}

func printTools(w io.Writer, tools []toolSummary, asJSON bool) error {
	if asJSON {
		return encodeJSON(w, tools)
	}
	for _, t := range tools {
		if _, err := fmt.Fprintf(w, "%s\n  %s\n\n", t.Name, t.Description); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// tools call
// ---------------------------------------------------------------------------

// ToolsCallCmd invokes any MCP tool by name with a JSON argument object. This
// is a generic escape hatch so callers can reach tools that do not have a
// dedicated subcommand (for example querying a data source's rows). Arguments
// are supplied as a JSON object via --args, --args-file, or stdin ("-").
type ToolsCallCmd struct {
	Name     string `arg:"" help:"MCP tool name (e.g. notion-query-data-sources)"`
	Args     string `help:"Tool arguments as a JSON object" short:"a" default:"{}"`
	ArgsFile string `help:"Read JSON arguments from a file, or '-' for stdin" short:"f"`
	Raw      bool   `help:"Print the full tool result as JSON (all content blocks + isError)" short:"r"`
}

func (c *ToolsCallCmd) Run(_ *Context) error {
	args, err := c.loadArgs(os.Stdin)
	if err != nil {
		output.PrintError(err)
		return err
	}

	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	result, err := client.CallTool(context.Background(), c.Name, args)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if err := renderToolResult(os.Stdout, c.Name, result, c.Raw); err != nil {
		output.PrintError(err)
		return err
	}
	return nil
}

// loadArgs resolves the JSON argument object from --args, or from --args-file
// (a path, or "-" to read stdin). stdin is injected so it is unit-testable.
func (c *ToolsCallCmd) loadArgs(stdin io.Reader) (map[string]any, error) {
	raw := c.Args
	if c.ArgsFile != "" {
		data, err := readArgsSource(c.ArgsFile, stdin)
		if err != nil {
			return nil, err
		}
		raw = data
	}

	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, &output.UserError{Message: "invalid JSON arguments: " + err.Error()}
	}
	return args, nil
}

func readArgsSource(path string, stdin io.Reader) (string, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		return string(data), err
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

// renderToolResult writes the tool result to w: raw mode emits the full result
// as JSON, otherwise the text content. A tool-reported error surfaces as a
// returned error (non-zero exit) in either mode.
func renderToolResult(w io.Writer, toolName string, result *mcp.CallToolResult, raw bool) error {
	if raw {
		if err := encodeJSON(w, result); err != nil {
			return err
		}
	} else if !isToolError(result) {
		if _, err := fmt.Fprintln(w, toolResultText(result)); err != nil {
			return err
		}
	}

	if isToolError(result) {
		text := toolResultText(result)
		if text == "" {
			text = "tool call failed"
		}
		return &output.UserError{Message: fmt.Sprintf("tool %q returned an error: %s", toolName, text)}
	}
	return nil
}

func isToolError(result *mcp.CallToolResult) bool {
	return result != nil && result.IsError
}

func toolResultText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	var b strings.Builder
	for _, content := range result.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// encodeJSON writes v as indented JSON, matching the CLI's JSON output style.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
