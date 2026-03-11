package tmuxorch

func DraftSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"goal"},
		"properties": map[string]any{
			"goal": map[string]any{
				"type":        "string",
				"description": "Natural language goal for the orchestration.",
			},
			"mode": map[string]any{
				"type":        "string",
				"description": "auto|same-task|split-task",
				"default":     "auto",
			},
			"execution_kind": map[string]any{
				"type":        "string",
				"description": "auto|modify|analyze",
				"default":     "auto",
			},
			"workers": map[string]any{
				"type":        "integer",
				"description": "Optional explicit worker count (1-32).",
			},
			"run_id": map[string]any{
				"type":        "string",
				"description": "Optional explicit run id. If omitted, server generates one.",
			},
		},
	}
}

func ReviseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"run_id", "feedback"},
		"properties": map[string]any{
			"run_id":   map[string]any{"type": "string"},
			"feedback": map[string]any{"type": "string"},
		},
	}
}

func RunSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"run_id"},
		"properties": map[string]any{
			"run_id":        map[string]any{"type": "string"},
			"reuse_session": map[string]any{"type": "boolean", "default": false},
			"launch_stagger_ms": map[string]any{
				"type":        "integer",
				"description": "Delay between launching workers (ms). Default 400, clamp 0..5000.",
			},
		},
	}
}

func ControlSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"run_id", "worker_id", "action"},
		"properties": map[string]any{
			"run_id":    map[string]any{"type": "string"},
			"worker_id": map[string]any{"type": "string"},
			"action": map[string]any{
				"type":        "string",
				"description": "stop|inject|restart",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "Required when action=inject; otherwise optional.",
			},
		},
	}
}

func StatusSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"run_id"},
		"properties": map[string]any{
			"run_id": map[string]any{"type": "string"},
			"include_pane": map[string]any{
				"type":        "boolean",
				"description": "Include tmux pane runtime metadata (default true).",
				"default":     true,
			},
			"include_pane_tail": map[string]any{
				"type":        "boolean",
				"description": "Include tmux pane tail output (default false).",
				"default":     false,
			},
			"pane_tail_lines": map[string]any{
				"type":        "integer",
				"description": "Number of pane tail lines to capture (default 20, clamp 5..200).",
			},
			"include_log_tail": map[string]any{
				"type":        "boolean",
				"description": "Include log file tail output (default false).",
				"default":     false,
			},
			"log_tail_lines": map[string]any{
				"type":        "integer",
				"description": "Number of log tail lines to read (default 20, clamp 5..200).",
			},
		},
	}
}

func CloseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"run_id"},
		"properties": map[string]any{
			"run_id": map[string]any{"type": "string"},
		},
	}
}
