package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"vibecraft/backend/internal/store"
)

type PreparedWorkspace struct {
	Mode          string
	WorkspacePath string
	BranchName    *string
	BaseRef       *string
	WorktreePath  *string
	Artifacts     []store.AgentRunArtifactInput
}

type Inspection struct {
	ModifiedCode bool
	Artifacts    []store.AgentRunArtifactInput
}

type gitWorktreeEntry struct {
	Path   string
	Branch string
	Locked bool
}

// Prepare 功能：为 agent run 解析 workspace 策略，并在支持 Git worktree 时分配隔离工作目录。
// 参数/返回：run 为目标 agent；返回准备后的 workspace 信息与 artifact 输入。
// 失败场景：关键字段缺失时返回 error；git/worktree 失败时降级为 shared_workspace 并写 artifact。
// 副作用：可能调用 git 创建或修复 worktree 目录。
func Prepare(ctx context.Context, run store.AgentRun) (PreparedWorkspace, error) {
	mode := strings.TrimSpace(run.WorkspaceMode)
	if mode == "" {
		mode = defaultMode(run.Intent)
	}
	prepared := PreparedWorkspace{Mode: mode, WorkspacePath: run.WorkspacePath}
	summary := fmt.Sprintf("workspace_mode=%s", mode)

	if mode != "git_worktree" {
		payload := mustJSON(map[string]any{"workspace_mode": mode, "workspace_path": run.WorkspacePath})
		prepared.Artifacts = append(prepared.Artifacts, store.AgentRunArtifactInput{Kind: "workspace_strategy", Title: "Workspace Strategy", Summary: pointer(summary), PayloadJSON: &payload})
		return prepared, nil
	}

	repoRoot, err := gitOutput(ctx, run.WorkspacePath, "rev-parse", "--show-toplevel")
	if err != nil {
		prepared.Mode = "shared_workspace"
		prepared.WorkspacePath = run.WorkspacePath
		msg := fmt.Sprintf("git_worktree 不可用，已降级为 shared_workspace：%v", err)
		payload := mustJSON(map[string]any{"workspace_mode": prepared.Mode, "workspace_path": prepared.WorkspacePath, "fallback_reason": err.Error()})
		prepared.Artifacts = append(prepared.Artifacts, store.AgentRunArtifactInput{Kind: "workspace_strategy", Title: "Workspace Strategy", Summary: &msg, PayloadJSON: &payload})
		return prepared, nil
	}
	baseRef, _ := gitOutput(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if strings.TrimSpace(baseRef) == "HEAD" {
		baseRef, _ = gitOutput(ctx, repoRoot, "rev-parse", "--short", "HEAD")
	}
	branchName := sanitizeBranch(fmt.Sprintf("vibecraft/%s/%s", trimID(run.OrchestrationID), trimID(run.ID)))
	worktreePath := filepath.Join(os.TempDir(), "vibecraft-worktrees", trimID(run.OrchestrationID), trimID(run.ID))
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		prepared.Mode = "shared_workspace"
		prepared.WorkspacePath = run.WorkspacePath
		msg := fmt.Sprintf("创建 worktree 目录失败，已降级为 shared_workspace：%v", err)
		payload := mustJSON(map[string]any{"workspace_mode": prepared.Mode, "workspace_path": prepared.WorkspacePath, "fallback_reason": err.Error()})
		prepared.Artifacts = append(prepared.Artifacts, store.AgentRunArtifactInput{Kind: "workspace_strategy", Title: "Workspace Strategy", Summary: &msg, PayloadJSON: &payload})
		return prepared, nil
	}
	if err := ensureGitWorktree(ctx, repoRoot, branchName, worktreePath); err != nil {
		prepared.Mode = "shared_workspace"
		prepared.WorkspacePath = run.WorkspacePath
		msg := fmt.Sprintf("git_worktree 分配失败，已降级为 shared_workspace：%v", err)
		payload := mustJSON(map[string]any{"workspace_mode": prepared.Mode, "workspace_path": prepared.WorkspacePath, "fallback_reason": err.Error()})
		prepared.Artifacts = append(prepared.Artifacts, store.AgentRunArtifactInput{Kind: "workspace_strategy", Title: "Workspace Strategy", Summary: &msg, PayloadJSON: &payload})
		return prepared, nil
	}
	prepared.WorkspacePath = worktreePath
	prepared.BranchName = &branchName
	prepared.BaseRef = pointer(strings.TrimSpace(baseRef))
	prepared.WorktreePath = &worktreePath
	msg := fmt.Sprintf("已分配 git_worktree：branch=%s base=%s", branchName, strings.TrimSpace(baseRef))
	payload := mustJSON(map[string]any{"workspace_mode": prepared.Mode, "workspace_path": prepared.WorkspacePath, "branch_name": branchName, "base_ref": strings.TrimSpace(baseRef), "worktree_path": worktreePath})
	prepared.Artifacts = append(prepared.Artifacts, store.AgentRunArtifactInput{Kind: "workspace_strategy", Title: "Workspace Strategy", Summary: &msg, PayloadJSON: &payload})
	return prepared, nil
}

// Inspect 功能：在 agent run 执行结束后检查 workspace 状态，并生成代码变更摘要 artifact。
// 参数/返回：run 为已结束 agent；返回修改标记与 artifact 输入。
// 失败场景：检查失败时返回 error，并由上层决定是否降级忽略。
// 副作用：调用 git 读取 worktree 状态。
func Inspect(ctx context.Context, run store.AgentRun) (Inspection, error) {
	inspection := Inspection{}
	target := strings.TrimSpace(run.WorkspacePath)
	if target == "" {
		return inspection, nil
	}
	repoRoot, err := gitOutput(ctx, target, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return inspection, nil
	}
	status, err := gitOutput(ctx, target, "status", "--porcelain")
	if err != nil {
		return inspection, nil
	}
	status = strings.TrimSpace(status)
	inspection.ModifiedCode = status != ""
	var summary string
	if inspection.ModifiedCode {
		diffStat, _ := gitOutput(ctx, target, "diff", "--stat", "--compact-summary")
		summary = strings.TrimSpace(diffStat)
		if summary == "" {
			summary = status
		}
	} else {
		summary = "未检测到文件改动。"
	}
	payload := mustJSON(map[string]any{"workspace_path": target, "modified_code": inspection.ModifiedCode, "git_status": status})
	kind := "verification_summary"
	title := "Workspace Inspection"
	if inspection.ModifiedCode {
		kind = "code_change_summary"
		title = "Code Change Summary"
	}
	inspection.Artifacts = append(inspection.Artifacts, store.AgentRunArtifactInput{Kind: kind, Title: title, Summary: &summary, PayloadJSON: &payload})
	return inspection, nil
}

func ensureGitWorktree(ctx context.Context, repoRoot, branchName, worktreePath string) error {
	entries, err := listGitWorktrees(ctx, repoRoot)
	if err != nil {
		return err
	}
	if existing := findWorktreeByPath(entries, worktreePath); existing != nil {
		if existing.Branch == branchName {
			if _, err := gitOutput(ctx, worktreePath, "rev-parse", "--show-toplevel"); err == nil {
				return nil
			}
		}
		_ = gitRun(ctx, repoRoot, "worktree", "remove", "--force", worktreePath)
		_ = gitRun(ctx, repoRoot, "worktree", "prune")
	}
	if existing := findWorktreeByBranch(entries, branchName); existing != nil {
		if existing.Path == worktreePath {
			if _, err := gitOutput(ctx, worktreePath, "rev-parse", "--show-toplevel"); err == nil {
				return nil
			}
		}
		_ = gitRun(ctx, repoRoot, "worktree", "remove", "--force", existing.Path)
		_ = gitRun(ctx, repoRoot, "worktree", "prune")
	}
	if info, err := os.Stat(worktreePath); err == nil && info.IsDir() {
		if err := os.RemoveAll(worktreePath); err != nil {
			return err
		}
	}

	args := []string{"worktree", "add"}
	if gitBranchExists(ctx, repoRoot, branchName) {
		args = append(args, worktreePath, branchName)
	} else {
		args = append(args, "-b", branchName, worktreePath, "HEAD")
	}
	return gitRun(ctx, repoRoot, args...)
}

func listGitWorktrees(ctx context.Context, repoRoot string) ([]gitWorktreeEntry, error) {
	out, err := gitOutput(ctx, repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	entries := make([]gitWorktreeEntry, 0)
	var current *gitWorktreeEntry
	flush := func() {
		if current == nil || strings.TrimSpace(current.Path) == "" {
			return
		}
		entries = append(entries, *current)
		current = nil
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current = &gitWorktreeEntry{Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))}
		case current == nil:
			continue
		case strings.HasPrefix(line, "branch "):
			branchRef := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			current.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
		case line == "locked":
			current.Locked = true
		case line == "":
			flush()
		}
	}
	flush()
	return entries, nil
}

func findWorktreeByPath(entries []gitWorktreeEntry, worktreePath string) *gitWorktreeEntry {
	for _, entry := range entries {
		if entry.Path == worktreePath {
			copy := entry
			return &copy
		}
	}
	return nil
}

func findWorktreeByBranch(entries []gitWorktreeEntry, branchName string) *gitWorktreeEntry {
	for _, entry := range entries {
		if entry.Branch == branchName {
			copy := entry
			return &copy
		}
	}
	return nil
}

func gitBranchExists(ctx context.Context, repoRoot, branchName string) bool {
	return gitRun(ctx, repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branchName) == nil
}

func gitRun(ctx context.Context, cwd string, args ...string) error {
	_, err := gitCombinedOutput(ctx, cwd, args...)
	return err
}

func gitOutput(ctx context.Context, cwd string, args ...string) (string, error) {
	out, err := gitCombinedOutput(ctx, cwd, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitCombinedOutput(ctx context.Context, cwd string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	fullArgs := append([]string{"-C", cwd}, args...)
	cmd := exec.CommandContext(cmdCtx, "git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func defaultMode(intent string) string {
	if strings.TrimSpace(intent) == "analyze" {
		return "read_only"
	}
	if strings.TrimSpace(intent) == "modify" {
		return "git_worktree"
	}
	return "shared_workspace"
}

var branchUnsafe = regexp.MustCompile(`[^a-zA-Z0-9/_-]+`)

func sanitizeBranch(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, " ", "-")
	v = branchUnsafe.ReplaceAllString(v, "-")
	v = strings.Trim(v, "-/")
	if v == "" {
		return "vibecraft/agent"
	}
	return v
}

func trimID(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 12 {
		return v[:12]
	}
	if v == "" {
		return "agent"
	}
	return v
}

func pointer(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
