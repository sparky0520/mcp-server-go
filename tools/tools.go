package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sparky0520/mcp-server-go/server"
)

func RegisterDefaultTools(s *server.Server) {
	s.RegisterTool(
		server.ToolDefinition{
			Name:        "hello_world",
			Description: "Generate a greeting message for a given name. Use this when the user asks to greet someone, say hello, or produce a simple greeting.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Optional name to greet.",
					},
				},
			},
		},
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args HelloArgs
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, fmt.Errorf("decoding arguments: %w", err)
				}
			}
			return Hello(args)
		},
	)

}
