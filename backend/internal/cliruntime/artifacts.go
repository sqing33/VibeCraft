package cliruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

type Summary struct {
	Status       string   `json:"status"`
	Summary      string   `json:"summary"`
	ModifiedCode bool     `json:"modified_code"`
	NextAction   string   `json:"next_action,omitempty"`
	KeyFiles     []string `json:"key_files,omitempty"`
}

type Session struct {
	ToolID    string `json:"tool_id"`
	SessionID string `json:"session_id"`
	Model     string `json:"model,omitempty"`
	Resumed   bool   `json:"resumed,omitempty"`
}

type artifactList struct {
	Artifacts []artifactItem `json:"artifacts"`
}

type artifactItem struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

func PrepareRunSpec(spec runner.RunSpec, artifactDir string) runner.RunSpec {
	if strings.TrimSpace(artifactDir) == "" {
		return spec
	}
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}
	spec.Env["VIBE_TREE_ARTIFACT_DIR"] = artifactDir
	if strings.TrimSpace(spec.Cwd) != "" {
		spec.Env["VIBE_TREE_WORKSPACE"] = spec.Cwd
	}
	return spec
}

func WorkflowNodeArtifactDir(workflowID, nodeID string) (string, error) {
	root, err := paths.CLIRuntimeArtifactsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "workflows", workflowID, nodeID), nil
}

func AgentRunArtifactDir(orchestrationID, agentRunID string) (string, error) {
	root, err := paths.CLIRuntimeArtifactsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "orchestrations", orchestrationID, agentRunID), nil
}

func ChatTurnArtifactDir(sessionID, messageID string) (string, error) {
	root, err := paths.CLIRuntimeArtifactsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "chat", sessionID, messageID), nil
}

func ReadSummary(dir string) (*Summary, error) {
	data, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		return nil, err
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("parse summary.json: %w", err)
	}
	return &summary, nil
}

func ReadSession(dir string) (*Session, error) {
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session.json: %w", err)
	}
	if strings.TrimSpace(session.SessionID) == "" {
		return nil, os.ErrNotExist
	}
	return &session, nil
}

func ReadFinalMessage(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "final_message.md"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func CollectArtifacts(dir string) ([]store.AgentRunArtifactInput, error) {
	items := make([]store.AgentRunArtifactInput, 0)
	summary, err := ReadSummary(dir)
	if err == nil && summary != nil {
		payload, _ := json.Marshal(summary)
		items = append(items, store.AgentRunArtifactInput{
			Kind:        "cli_session_summary",
			Title:       "CLI Session Summary",
			Summary:     pointer(summary.Summary),
			PayloadJSON: pointerString(string(payload)),
		})
	}
	if data, err := os.ReadFile(filepath.Join(dir, "artifacts.json")); err == nil {
		var manifest artifactList
		if json.Unmarshal(data, &manifest) == nil {
			for _, item := range manifest.Artifacts {
				if strings.TrimSpace(item.Kind) == "" {
					continue
				}
				payload, _ := json.Marshal(item.Payload)
				var payloadRef *string
				if len(payload) > 0 && string(payload) != "null" {
					payloadRef = pointerString(string(payload))
				}
				items = append(items, store.AgentRunArtifactInput{
					Kind:        strings.TrimSpace(item.Kind),
					Title:       firstNonEmpty(strings.TrimSpace(item.Title), strings.TrimSpace(item.Kind)),
					Summary:     pointer(strings.TrimSpace(item.Summary)),
					PayloadJSON: payloadRef,
				})
			}
		}
	}
	if len(items) == 0 {
		return nil, os.ErrNotExist
	}
	return items, nil
}

func SummaryText(dir string) *string {
	if summary, err := ReadSummary(dir); err == nil && summary != nil && strings.TrimSpace(summary.Summary) != "" {
		return pointer(summary.Summary)
	}
	if final, err := ReadFinalMessage(dir); err == nil && strings.TrimSpace(final) != "" {
		return pointer(final)
	}
	return nil
}

func pointer(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	vv := v
	return &vv
}

func pointerString(v string) *string {
	vv := v
	return &vv
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
