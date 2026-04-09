package monday

import (
	"encoding/json"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

// mondayServerJSON returns the standalone MCP server config for Claude Code
// (StrategySeparateMCPFiles: one JSON file per server).
func mondayServerJSON(token string) []byte {
	cfg := map[string]any{
		"command": "npx",
		"args":    []string{"-y", "@mondaydotcomorg/monday-ai-mcp"},
		"env": map[string]string{
			"MONDAY_API_TOKEN": token,
		},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return append(b, '\n')
}

// mondayOverlayJSON returns the settings overlay for agents that merge MCP
// config into a single settings file (OpenCode, Gemini CLI, Cursor, etc.).
func mondayOverlayJSON(agentID model.AgentID, token string) []byte {
	var cfg map[string]any

	switch agentID {
	case model.AgentOpenCode:
		cfg = map[string]any{
			"mcp": map[string]any{
				"monday": map[string]any{
					"command": []string{"npx", "-y", "@mondaydotcomorg/monday-ai-mcp"},
					"type":    "local",
					"env": map[string]string{
						"MONDAY_API_TOKEN": token,
					},
				},
			},
		}
	case model.AgentVSCodeCopilot:
		cfg = map[string]any{
			"servers": map[string]any{
				"monday": map[string]any{
					"command": "npx",
					"args":    []string{"-y", "@mondaydotcomorg/monday-ai-mcp"},
					"env": map[string]string{
						"MONDAY_API_TOKEN": token,
					},
				},
			},
		}
	case model.AgentAntigravity:
		cfg = map[string]any{
			"mcpServers": map[string]any{
				"monday": map[string]any{
					"command": "npx",
					"args":    []string{"-y", "@mondaydotcomorg/monday-ai-mcp"},
					"env": map[string]string{
						"MONDAY_API_TOKEN": token,
					},
				},
			},
		}
	default:
		// Default mcpServers key (Gemini CLI and others).
		cfg = map[string]any{
			"mcpServers": map[string]any{
				"monday": map[string]any{
					"command": "npx",
					"args":    []string{"-y", "@mondaydotcomorg/monday-ai-mcp"},
					"env": map[string]string{
						"MONDAY_API_TOKEN": token,
					},
				},
			},
		}
	}

	b, _ := json.MarshalIndent(cfg, "", "  ")
	return append(b, '\n')
}
