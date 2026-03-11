package main

import (
	"encoding/json"
	"fmt"
	"os"

	"vibe-tree/backend/internal/mcpstdio"
	"vibe-tree/backend/internal/tmuxorch"
)

func main() {
	orchestrator := tmuxorch.New()
	server := mcpstdio.NewServer("vt-mcp-tmux-orch", "0.1.0")

	server.RegisterTool(mcpstdio.Tool{
		Name:        "tmux_orch_doctor",
		Description: "Check required tools (git/tmux/codex) and return orchestrator paths.",
		InputSchema: mcpstdio.EmptyObjectSchema(),
		Handler: func(_ json.RawMessage) (any, error) {
			return orchestrator.Doctor()
		},
	})
	server.RegisterTool(mcpstdio.Tool{
		Name:        "tmux_orch_draft",
		Description: "Create a new orchestration run state and ORCH_PLAN.md.",
		InputSchema: tmuxorch.DraftSchema(),
		Handler: func(args json.RawMessage) (any, error) {
			var req tmuxorch.DraftRequest
			if len(args) > 0 {
				if err := json.Unmarshal(args, &req); err != nil {
					return nil, err
				}
			}
			return orchestrator.Draft(req)
		},
	})
	server.RegisterTool(mcpstdio.Tool{
		Name:        "tmux_orch_revise",
		Description: "Revise a run plan/state based on feedback.",
		InputSchema: tmuxorch.ReviseSchema(),
		Handler: func(args json.RawMessage) (any, error) {
			var req tmuxorch.ReviseRequest
			if err := json.Unmarshal(args, &req); err != nil {
				return nil, err
			}
			return orchestrator.Revise(req)
		},
	})
	server.RegisterTool(mcpstdio.Tool{
		Name:        "tmux_orch_run",
		Description: "Launch tmux session and start worker Codex processes.",
		InputSchema: tmuxorch.RunSchema(),
		Handler: func(args json.RawMessage) (any, error) {
			var req tmuxorch.RunRequest
			if err := json.Unmarshal(args, &req); err != nil {
				return nil, err
			}
			return orchestrator.Run(req)
		},
	})
	server.RegisterTool(mcpstdio.Tool{
		Name:        "tmux_orch_control",
		Description: "Control a worker: stop, inject, restart.",
		InputSchema: tmuxorch.ControlSchema(),
		Handler: func(args json.RawMessage) (any, error) {
			var req tmuxorch.ControlRequest
			if err := json.Unmarshal(args, &req); err != nil {
				return nil, err
			}
			return orchestrator.Control(req)
		},
	})
	server.RegisterTool(mcpstdio.Tool{
		Name:        "tmux_orch_status",
		Description: "Refresh and return run status.",
		InputSchema: tmuxorch.StatusSchema(),
		Handler: func(args json.RawMessage) (any, error) {
			var req tmuxorch.StatusRequest
			if err := json.Unmarshal(args, &req); err != nil {
				return nil, err
			}
			return orchestrator.Status(req)
		},
	})
	server.RegisterTool(mcpstdio.Tool{
		Name:        "tmux_orch_close",
		Description: "Close tmux session for a run (if active).",
		InputSchema: tmuxorch.CloseSchema(),
		Handler: func(args json.RawMessage) (any, error) {
			var req tmuxorch.CloseRequest
			if err := json.Unmarshal(args, &req); err != nil {
				return nil, err
			}
			return orchestrator.Close(req)
		},
	})

	if err := server.ServeStdio(os.Stdin, os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
