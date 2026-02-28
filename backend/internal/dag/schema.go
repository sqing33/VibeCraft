package dag

// JSONSchemaV1 功能：返回 master 规划输出（DAG）的 JSON Schema（用于 SDK structured output）。
// 参数/返回：无入参；返回可直接用于 OpenAI/Anthropic JSON schema 输出的 map。
// 失败场景：无。
// 副作用：无。
func JSONSchemaV1() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"workflow_title": map[string]any{
				"type": "string",
			},
			"nodes": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"id": map[string]any{
							"type": "string",
						},
						"title": map[string]any{
							"type": "string",
						},
						"type": map[string]any{
							"type": "string",
						},
						"expert_id": map[string]any{
							"type": "string",
						},
						"fallback_expert_id": map[string]any{
							"type": "string",
						},
						"complexity": map[string]any{
							"type": "string",
						},
						"quality_tier": map[string]any{
							"type": "string",
						},
						"model": map[string]any{
							"type": []any{"string", "null"},
						},
						"routing_reason": map[string]any{
							"type": "string",
						},
						"prompt": map[string]any{
							"type": "string",
						},
					},
					"required": []any{"id", "expert_id", "prompt"},
				},
			},
			"edges": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"from": map[string]any{
							"type": "string",
						},
						"to": map[string]any{
							"type": "string",
						},
						"type": map[string]any{
							"type": "string",
						},
						"source_handle": map[string]any{
							"type": []any{"string", "null"},
						},
						"target_handle": map[string]any{
							"type": []any{"string", "null"},
						},
					},
					"required": []any{"from", "to"},
				},
			},
		},
		"required": []any{"nodes"},
	}
}
