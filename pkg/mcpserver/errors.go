package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

type toolError struct {
	Kind    string `json:"kind"`
	Host    string `json:"host,omitempty"`
	Task    string `json:"task,omitempty"`
	Message string `json:"message"`
}

func newToolError(err error) toolError {
	te := toolError{Kind: fmt.Sprintf("%T", err), Message: err.Error()}
	var ht myerrors.HostTask
	if errors.As(err, &ht) {
		te.Host, te.Task = ht.Host, ht.Task
		if ht.Err != nil {
			te.Kind = fmt.Sprintf("%T", ht.Err)
		}
	}
	return te
}

// errorResult keeps the partial result in StructuredContent, which returning a plain error from the handler would drop.
func errorResult(err error) *mcp.CallToolResult {
	body, _ := json.Marshal(newToolError(err))
	res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}
	res.IsError = true
	return res
}
