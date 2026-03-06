package expertschema

// ExpertBuilderJSONSchemaV1 功能：返回专家生成结果的 JSON Schema，用于约束 builder 输出结构。
// 参数/返回：无入参；返回 schema map。
// 失败场景：无。
// 副作用：无。
func ExpertBuilderJSONSchemaV1() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"assistant_message", "expert"},
		"properties": map[string]any{
			"assistant_message": map[string]any{"type": "string"},
			"expert": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []any{"id", "label", "description", "category", "primary_model_id", "system_prompt"},
				"properties": map[string]any{
					"id":                 map[string]any{"type": "string"},
					"label":              map[string]any{"type": "string"},
					"description":        map[string]any{"type": "string"},
					"category":           map[string]any{"type": "string"},
					"avatar":             map[string]any{"type": "string"},
					"primary_model_id":   map[string]any{"type": "string"},
					"secondary_model_id": map[string]any{"type": "string"},
					"system_prompt":      map[string]any{"type": "string"},
					"prompt_template":    map[string]any{"type": "string"},
					"output_format":      map[string]any{"type": "string"},
					"max_output_tokens":  map[string]any{"type": "integer", "minimum": 0},
					"timeout_ms":         map[string]any{"type": "integer", "minimum": 0},
					"temperature":        map[string]any{"type": "number"},
					"enabled_skills": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
					"fallback_on": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}
