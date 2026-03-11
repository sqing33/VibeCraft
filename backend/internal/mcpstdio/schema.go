package mcpstdio

// Minimal JSON schema helpers for MCP tool input.

func EmptyObjectSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           map[string]any{},
	}
}
