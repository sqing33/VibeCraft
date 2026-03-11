package tmuxorch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultVerifyCmd = "openspec validate --all"
)

var (
	directExecKeywords  = []string{"直接执行", "不用表格", "跳过表格", "无需审查", "直接开跑", "马上执行"}
	analyzeOnlyKeywords = []string{"只分析", "仅分析", "只读", "不修改", "不要修改", "不进行实际修改", "分析分支"}
	modifyKeywords      = []string{"开始修改", "允许修改", "可以修改", "实际修改", "动手改"}

	sameTaskStrategies = []string{"balanced", "conservative", "performance", "refactor", "test-heavy", "security-first", "minimal-diff", "creative"}
)

type Orchestrator struct{}

func New() *Orchestrator { return &Orchestrator{} }

type DoctorResult struct {
	RepoRoot     string   `json:"repo_root"`
	ToolRoot     string   `json:"tool_root"`
	StateDir     string   `json:"state_dir"`
	MissingTools []string `json:"missing_tools"`
}

func (o *Orchestrator) Doctor() (DoctorResult, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return DoctorResult{}, err
	}
	toolRoot := filepath.Join(repoRoot, ".codex", "tools", "tmux-orch")
	stateDir := filepath.Join(toolRoot, "state")
	needed := []string{"git", "tmux", "codex", "bash"}
	var missing []string
	for _, tool := range needed {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	return DoctorResult{
		RepoRoot:     repoRoot,
		ToolRoot:     toolRoot,
		StateDir:     stateDir,
		MissingTools: missing,
	}, nil
}

type DraftResult struct {
	RunID string    `json:"run_id"`
	State RunState  `json:"state"`
	Plan  string    `json:"plan_path"`
	Now   time.Time `json:"now"`
}

func (o *Orchestrator) Draft(req DraftRequest) (DraftResult, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return DraftResult{}, err
	}
	if strings.TrimSpace(req.Goal) == "" {
		return DraftResult{}, fmt.Errorf("goal must not be empty")
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		runID = runIDNow()
	}
	mode := detectMode(req.Goal, strings.TrimSpace(req.Mode))
	executionKind := detectExecutionKind(req.Goal, strings.TrimSpace(req.ExecutionKind))
	tasks := splitGoalTasks(req.Goal)
	workers := decideWorkers(mode, req.Goal, tasks, req.Workers)
	baseBranch, err := gitCurrentBranch(repoRoot)
	if err != nil {
		return DraftResult{}, err
	}

	state := NewRunState(repoRoot, runID, req.Goal, mode, executionKind, baseBranch, workers, tasks)
	if detectDirectExecution(req.Goal) {
		state.ExecutionPolicy = "direct"
	}

	if err := ensureDirs(state); err != nil {
		return DraftResult{}, err
	}
	if err := saveState(state); err != nil {
		return DraftResult{}, err
	}
	if err := writePlan(state); err != nil {
		return DraftResult{}, err
	}
	return DraftResult{
		RunID: runID,
		State: *state,
		Plan:  planPath(state),
		Now:   time.Now(),
	}, nil
}

type ReviseResult struct {
	RunID string   `json:"run_id"`
	State RunState `json:"state"`
	Plan  string   `json:"plan_path"`
}

func (o *Orchestrator) Revise(req ReviseRequest) (ReviseResult, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return ReviseResult{}, err
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return ReviseResult{}, fmt.Errorf("run_id is required")
	}
	state, err := loadState(repoRoot, runID)
	if err != nil {
		return ReviseResult{}, err
	}
	if strings.TrimSpace(req.Feedback) == "" {
		return ReviseResult{}, fmt.Errorf("feedback must not be empty")
	}
	feedback := req.Feedback

	// Resize only applies to same-task.
	if state.Mode == "same-task" {
		if count := parseExplicitWorkers(feedback); count != nil {
			resizeSameTaskWorkers(state, *count)
		}
	}

	if detectDirectExecution(feedback) {
		state.ExecutionPolicy = "direct"
	}
	if detectAnalyzeOnly(feedback) {
		state.ExecutionKind = "analyze"
	} else if detectModifyIntent(feedback) {
		state.ExecutionKind = "modify"
	}
	applyExecutionKindToRows(state)

	if err := ensureDirs(state); err != nil {
		return ReviseResult{}, err
	}
	if err := saveState(state); err != nil {
		return ReviseResult{}, err
	}
	if err := writePlan(state); err != nil {
		return ReviseResult{}, err
	}
	return ReviseResult{RunID: runID, State: *state, Plan: planPath(state)}, nil
}

type RunResult struct {
	RunID    string   `json:"run_id"`
	Session  string   `json:"session"`
	Launched int      `json:"launched"`
	State    RunState `json:"state"`
}

func (o *Orchestrator) Run(req RunRequest) (RunResult, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return RunResult{}, err
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return RunResult{}, fmt.Errorf("run_id is required")
	}
	state, err := loadState(repoRoot, runID)
	if err != nil {
		return RunResult{}, err
	}
	applyExecutionKindToRows(state)
	refreshWorkerStatuses(state)

	sessionName := state.SessionName
	if sessionName == "" {
		sessionName = ("orch-" + state.RunID)
		if len(sessionName) > 40 {
			sessionName = sessionName[:40]
		}
		state.SessionName = sessionName
	}
	if tmuxHasSession(sessionName) && !req.ReuseSession {
		return RunResult{}, fmt.Errorf("tmux session already exists: %s", sessionName)
	}

	firstPane := ""
	if !tmuxHasSession(sessionName) {
		p, err := tmuxNewSession(sessionName)
		if err != nil {
			return RunResult{}, err
		}
		firstPane = p
	}

	staggerMs := clamp(intOr(req.LaunchStaggerMs, 400), 0, 5000)
	toLaunch := 0
	for _, row := range state.Workers {
		if row.Status != "done" {
			toLaunch++
		}
	}

	launched := 0
	for i := range state.Workers {
		row := &state.Workers[i]
		if row.Status == "done" {
			continue
		}

		if state.ExecutionKind == "modify" {
			if err := ensureWorkerWorktree(repoRoot, row); err != nil {
				return RunResult{}, err
			}
		}

		paneID := ""
		if firstPane != "" {
			paneID = firstPane
			firstPane = ""
		} else if row.PaneID != "" && tmuxPaneExists(row.PaneID) {
			paneID = row.PaneID
		} else {
			p, err := tmuxNewPane(sessionName)
			if err != nil {
				return RunResult{}, err
			}
			paneID = p
		}

		prompt := workerPrompt(state, row)
		if err := startWorker(repoRoot, state, row, paneID, prompt, false); err != nil {
			return RunResult{}, err
		}
		launched++
		toLaunch--
		if staggerMs > 0 && toLaunch > 0 {
			time.Sleep(time.Duration(staggerMs) * time.Millisecond)
		}
	}

	if err := saveState(state); err != nil {
		return RunResult{}, err
	}
	if err := writePlan(state); err != nil {
		return RunResult{}, err
	}
	return RunResult{RunID: runID, Session: sessionName, Launched: launched, State: *state}, nil
}

type ControlResult struct {
	RunID    string   `json:"run_id"`
	WorkerID string   `json:"worker_id"`
	Action   string   `json:"action"`
	State    RunState `json:"state"`
}

func (o *Orchestrator) Control(req ControlRequest) (ControlResult, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return ControlResult{}, err
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return ControlResult{}, fmt.Errorf("run_id is required")
	}
	state, err := loadState(repoRoot, runID)
	if err != nil {
		return ControlResult{}, err
	}
	applyExecutionKindToRows(state)
	refreshWorkerStatuses(state)

	workerID := strings.TrimSpace(req.WorkerID)
	row, err := rowByWorkerID(state, workerID)
	if err != nil {
		return ControlResult{}, err
	}

	action := strings.TrimSpace(req.Action)
	switch action {
	case "stop":
		if row.PaneID != "" && tmuxPaneExists(row.PaneID) {
			_ = tmuxCtrlC(row.PaneID)
		}
		row.Status = "blocked"
		row.Notes = appendNote(row.Notes, "manual_stop")
	case "inject", "restart":
		if action == "inject" && strings.TrimSpace(req.Prompt) == "" {
			return ControlResult{}, fmt.Errorf("prompt is required for action=inject")
		}
		paneID, err := ensureWorkerPane(state, row)
		if err != nil {
			return ControlResult{}, err
		}
		_ = tmuxCtrlC(paneID)
		prompt := strings.TrimSpace(req.Prompt)
		useResume := action == "inject"
		if prompt == "" {
			prompt = workerPrompt(state, row)
		}
		if err := startWorker(repoRoot, state, row, paneID, prompt, useResume); err != nil {
			return ControlResult{}, err
		}
		row.Notes = appendNote(row.Notes, "manual_"+action)
	default:
		return ControlResult{}, fmt.Errorf("unsupported action: %s", action)
	}

	if err := saveState(state); err != nil {
		return ControlResult{}, err
	}
	if err := writePlan(state); err != nil {
		return ControlResult{}, err
	}
	return ControlResult{RunID: runID, WorkerID: workerID, Action: action, State: *state}, nil
}

type StatusResult struct {
	RunID   string         `json:"run_id"`
	Counts  map[string]int `json:"status_counts"`
	Workers []WorkerRow    `json:"workers"`
	State   RunState       `json:"state"`

	Runtime []WorkerRuntime `json:"runtime,omitempty"`
}

func (o *Orchestrator) Status(req StatusRequest) (StatusResult, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return StatusResult{}, err
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return StatusResult{}, fmt.Errorf("run_id is required")
	}
	state, err := loadState(repoRoot, runID)
	if err != nil {
		return StatusResult{}, err
	}
	applyExecutionKindToRows(state)
	refreshWorkerStatuses(state)
	counts := workerStatusCounter(state.Workers)

	runtime, err := buildWorkerRuntime(repoRoot, state, req)
	if err != nil {
		return StatusResult{}, err
	}

	if err := saveState(state); err != nil {
		return StatusResult{}, err
	}
	if err := writePlan(state); err != nil {
		return StatusResult{}, err
	}
	return StatusResult{
		RunID:   runID,
		Counts:  counts,
		Workers: append([]WorkerRow(nil), state.Workers...),
		State:   *state,
		Runtime: runtime,
	}, nil
}

type CloseResult struct {
	RunID string   `json:"run_id"`
	Msg   string   `json:"message"`
	State RunState `json:"state"`
}

func (o *Orchestrator) Close(req CloseRequest) (CloseResult, error) {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return CloseResult{}, err
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return CloseResult{}, fmt.Errorf("run_id is required")
	}
	state, err := loadState(repoRoot, runID)
	if err != nil {
		return CloseResult{}, err
	}
	sessionName := strings.TrimSpace(state.SessionName)
	msg := "no_active_session"
	if sessionName != "" && tmuxHasSession(sessionName) {
		_ = tmuxKillSession(sessionName)
		msg = "closed_session=" + sessionName
	}
	if err := saveState(state); err != nil {
		return CloseResult{}, err
	}
	if err := writePlan(state); err != nil {
		return CloseResult{}, err
	}
	return CloseResult{RunID: runID, Msg: msg, State: *state}, nil
}

type WorkerRuntime struct {
	WorkerID string `json:"worker_id"`

	Pane     *PaneRuntime `json:"pane,omitempty"`
	PaneTail []string     `json:"pane_tail,omitempty"`

	LogPath string   `json:"log_path,omitempty"`
	LogTail []string `json:"log_tail,omitempty"`

	DonePath   string `json:"done_path,omitempty"`
	DoneExists bool   `json:"done_exists,omitempty"`
}

type PaneRuntime struct {
	ID             string `json:"id"`
	Exists         bool   `json:"exists"`
	PID            int    `json:"pid,omitempty"`
	CurrentCommand string `json:"current_command,omitempty"`
	Active         bool   `json:"active,omitempty"`
	Dead           bool   `json:"dead,omitempty"`
	Error          string `json:"error,omitempty"`
}

func boolOr(ptr *bool, def bool) bool {
	if ptr == nil {
		return def
	}
	return *ptr
}

func intOr(ptr *int, def int) int {
	if ptr == nil {
		return def
	}
	return *ptr
}

func clampLines(n int) int {
	if n < 5 {
		return 5
	}
	if n > 200 {
		return 200
	}
	return n
}

func buildWorkerRuntime(repoRoot string, state *RunState, req StatusRequest) ([]WorkerRuntime, error) {
	includePane := boolOr(req.IncludePane, true)
	includePaneTail := boolOr(req.IncludePaneTail, false)
	includeLogTail := boolOr(req.IncludeLogTail, false)
	paneTailLines := clampLines(intOr(req.PaneTailLines, 20))
	logTailLines := clampLines(intOr(req.LogTailLines, 20))

	if !includePane && !includeLogTail && !includePaneTail {
		return nil, nil
	}

	out := make([]WorkerRuntime, 0, len(state.Workers))
	for _, row := range state.Workers {
		rt := WorkerRuntime{
			WorkerID: row.WorkerID,
		}
		if row.LogFile != "" {
			rt.LogPath = row.LogFile
		}
		if row.DoneFile != "" {
			rt.DonePath = row.DoneFile
			if st, err := os.Stat(row.DoneFile); err == nil && !st.IsDir() {
				rt.DoneExists = true
			}
		}

		if includePane {
			rt.Pane = inspectPane(row.PaneID)
			if includePaneTail && rt.Pane != nil && rt.Pane.Exists {
				if tail, err := capturePaneTail(row.PaneID, paneTailLines); err == nil {
					rt.PaneTail = tail
				} else if rt.Pane.Error == "" {
					rt.Pane.Error = err.Error()
				}
			}
		} else if includePaneTail {
			// If caller explicitly wants pane tail, we still need to check existence.
			p := inspectPane(row.PaneID)
			rt.Pane = p
			if p != nil && p.Exists {
				if tail, err := capturePaneTail(row.PaneID, paneTailLines); err == nil {
					rt.PaneTail = tail
				} else if p.Error == "" {
					p.Error = err.Error()
				}
			}
		}

		if includeLogTail && row.LogFile != "" {
			if tail, err := tailFileLines(row.LogFile, logTailLines); err == nil {
				rt.LogTail = tail
			}
		}

		out = append(out, rt)
	}

	return out, nil
}

func inspectPane(paneID string) *PaneRuntime {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return &PaneRuntime{ID: "", Exists: false}
	}
	if !tmuxPaneExists(paneID) {
		return &PaneRuntime{ID: paneID, Exists: false}
	}

	// Format: pid cmd active dead
	out, err := sh("", "tmux", "display-message", "-p", "-t", paneID, "#{pane_pid} #{pane_current_command} #{pane_active} #{pane_dead}")
	if err != nil {
		return &PaneRuntime{ID: paneID, Exists: true, Error: err.Error()}
	}
	parts := strings.Fields(strings.TrimSpace(out))
	rt := &PaneRuntime{ID: paneID, Exists: true}
	if len(parts) >= 1 {
		if pid, _ := strconv.Atoi(parts[0]); pid > 0 {
			rt.PID = pid
		}
	}
	if len(parts) >= 2 {
		rt.CurrentCommand = parts[1]
	}
	if len(parts) >= 3 {
		rt.Active = parts[2] == "1"
	}
	if len(parts) >= 4 {
		rt.Dead = parts[3] == "1"
	}
	return rt
}

func capturePaneTail(paneID string, lines int) ([]string, error) {
	if strings.TrimSpace(paneID) == "" {
		return nil, fmt.Errorf("pane_id is empty")
	}
	if lines <= 0 {
		return nil, nil
	}
	// `-S -N` means start N lines from the bottom.
	out, err := sh("", "tmux", "capture-pane", "-p", "-t", paneID, "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		return nil, err
	}
	return splitLinesTrimRight(out), nil
}

func splitLinesTrimRight(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if strings.TrimSpace(s) == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Trim trailing carriage returns for safety.
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

func tailFileLines(path string, n int) ([]string, error) {
	n = clampLines(n)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Read at most the last 256KB to avoid huge logs.
	const maxRead = 256 * 1024
	size := st.Size()
	var start int64
	if size > maxRead {
		start = size - maxRead
	}
	if start > 0 {
		if _, err := f.Seek(start, 0); err != nil {
			return nil, err
		}
	}

	// If we started mid-file, discard the first partial line.
	if start > 0 {
		r := bufio.NewReader(f)
		_, _ = r.ReadString('\n')
		// Continue scanning from the reader; wrap it back into scanner.
		return tailLinesFromReader(r, n)
	}
	return tailLinesFromReader(f, n)
}

func tailLinesFromReader(r io.Reader, n int) ([]string, error) {
	sc := bufio.NewScanner(r)
	// Allow longer lines.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 2*1024*1024)
	ring := make([]string, 0, n)
	for sc.Scan() {
		line := sc.Text()
		if len(ring) < n {
			ring = append(ring, line)
			continue
		}
		copy(ring, ring[1:])
		ring[n-1] = line
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// Drop fully empty tail.
	allEmpty := true
	for _, l := range ring {
		if strings.TrimSpace(l) != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return nil, nil
	}
	return ring, nil
}

// ----- State model + persistence -----

type RunState struct {
	RunID           string      `json:"run_id"`
	Goal            string      `json:"goal"`
	Mode            string      `json:"mode"`
	ExecutionKind   string      `json:"execution_kind"`
	ExecutionPolicy string      `json:"execution_policy"`
	BaseBranch      string      `json:"base_branch"`
	SessionName     string      `json:"session_name"`
	CreatedAt       string      `json:"created_at"`
	UpdatedAt       string      `json:"updated_at"`
	Workers         []WorkerRow `json:"workers"`
}

type WorkerRow struct {
	RunID        string `json:"run_id"`
	Mode         string `json:"mode"`
	WorkerID     string `json:"worker_id"`
	TaskTitle    string `json:"task_title"`
	TaskScope    string `json:"task_scope"`
	Strategy     string `json:"strategy"`
	BaseBranch   string `json:"base_branch"`
	WorkerBranch string `json:"worker_branch"`
	WorktreePath string `json:"worktree_path"`
	VerifyCmd    string `json:"verify_cmd"`
	Status       string `json:"status"`
	SessionID    string `json:"session_id"`
	ResultRef    string `json:"result_ref"`
	Notes        string `json:"notes"`
	PaneID       string `json:"pane_id,omitempty"`
	PromptFile   string `json:"prompt_file,omitempty"`
	ScriptFile   string `json:"script_file,omitempty"`
	LogFile      string `json:"log_file,omitempty"`
	DoneFile     string `json:"done_file,omitempty"`
}

func NewRunState(repoRoot, runID, goal, mode, executionKind, baseBranch string, workers int, tasks []string) *RunState {
	rows := buildWorkerRows(repoRoot, runID, mode, executionKind, goal, tasks, workers, baseBranch)
	state := &RunState{
		RunID:           runID,
		Goal:            goal,
		Mode:            mode,
		ExecutionKind:   executionKind,
		ExecutionPolicy: "review_first",
		BaseBranch:      baseBranch,
		SessionName:     "",
		CreatedAt:       isoNow(),
		UpdatedAt:       isoNow(),
		Workers:         rows,
	}
	if state.SessionName == "" {
		name := "orch-" + runID
		if len(name) > 40 {
			name = name[:40]
		}
		state.SessionName = name
	}
	return state
}

func isoNow() string {
	return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
}

func toolRoot(repoRoot string) string {
	return ToolRootForRepo(repoRoot)
}

func stateFile(repoRoot, runID string) string {
	return StateFileForRepo(repoRoot, runID)
}

func planPath(state *RunState) string {
	return filepath.Join(toolRootFromState(state), "plans", "ORCH_PLAN.md")
}

func toolRootFromState(state *RunState) string {
	// Derived at runtime from git repo root; state doesn't store it.
	// planPath callers already have repo root via stateFile location.
	// Keep this simple by recomputing.
	// Note: state.BaseBranch is not a filesystem path.
	repoRoot, _ := gitRepoRoot()
	return toolRoot(repoRoot)
}

func ensureDirs(state *RunState) error {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return err
	}
	root := toolRoot(repoRoot)
	dirs := []string{
		filepath.Join(root, "state"),
		filepath.Join(root, "plans"),
		filepath.Join(root, "logs"),
		filepath.Join(root, "results"),
		filepath.Join(root, "state", state.RunID),
		filepath.Join(root, "logs", state.RunID),
		filepath.Join(root, "results", state.RunID),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func loadState(repoRoot, runID string) (*RunState, error) {
	path := stateFile(repoRoot, runID)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st RunState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func saveState(state *RunState) error {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return err
	}
	state.UpdatedAt = isoNow()
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile(repoRoot, state.RunID), append(b, '\n'), 0o644)
}

// ----- Plan rendering -----

var tableColumns = []string{
	"run_id",
	"mode",
	"worker_id",
	"task_title",
	"task_scope",
	"strategy",
	"base_branch",
	"worker_branch",
	"worktree_path",
	"verify_cmd",
	"status",
	"session_id",
	"result_ref",
	"notes",
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

func writePlan(state *RunState) error {
	repoRoot, err := gitRepoRoot()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("# ORCH_PLAN\n\n")
	buf.WriteString(fmt.Sprintf("- run_id: `%s`\n", state.RunID))
	buf.WriteString(fmt.Sprintf("- goal: %s\n", state.Goal))
	buf.WriteString(fmt.Sprintf("- mode: `%s`\n", state.Mode))
	buf.WriteString(fmt.Sprintf("- execution_kind: `%s`\n", state.ExecutionKind))
	buf.WriteString(fmt.Sprintf("- execution_policy: `%s`\n", state.ExecutionPolicy))
	buf.WriteString(fmt.Sprintf("- base_branch: `%s`\n", state.BaseBranch))
	buf.WriteString(fmt.Sprintf("- created_at: `%s`\n\n", state.CreatedAt))

	buf.WriteString("| " + strings.Join(tableColumns, " | ") + " |\n")
	buf.WriteString("| " + strings.Join(repeat("---", len(tableColumns)), " | ") + " |\n")
	for _, w := range state.Workers {
		row := []string{
			w.RunID,
			w.Mode,
			w.WorkerID,
			w.TaskTitle,
			w.TaskScope,
			w.Strategy,
			w.BaseBranch,
			w.WorkerBranch,
			w.WorktreePath,
			w.VerifyCmd,
			w.Status,
			w.SessionID,
			w.ResultRef,
			w.Notes,
		}
		for i := range row {
			row[i] = escapeCell(firstNonEmpty(row[i], "-"))
		}
		buf.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	buf.WriteString("\n## Notes\n\n")
	buf.WriteString("- Default policy is review-first; if goal/feedback includes direct-exec keywords, execution_policy becomes direct.\n")
	buf.WriteString("- execution_kind=analyze: no worktree creation; workers run codex in read-only sandbox.\n")
	buf.WriteString("\n")
	return os.WriteFile(filepath.Join(toolRoot(repoRoot), "plans", "ORCH_PLAN.md"), buf.Bytes(), 0o644)
}

func repeat(s string, n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, s)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ----- Draft helpers -----

func runIDNow() string {
	return time.Now().Format("20060102-150405") + fmt.Sprintf("-%04x", rand.Uint32()&0xFFFF)
}

func detectMode(goal, mode string) string {
	if mode != "" && mode != "auto" {
		return mode
	}
	sameTaskMarkers := []string{"同题", "同一个", "同样的修改", "多解", "多个方案", "same task"}
	for _, m := range sameTaskMarkers {
		if strings.Contains(strings.ToLower(goal), strings.ToLower(m)) {
			return "same-task"
		}
	}
	return "split-task"
}

func splitGoalTasks(goal string) []string {
	var bullets []string
	lines := strings.Split(goal, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "+") || regexp.MustCompile(`^\d+[\.)]`).MatchString(line) {
			text := regexp.MustCompile(`^[-*+\s]+`).ReplaceAllString(line, "")
			text = regexp.MustCompile(`^\d+[\.)]\s*`).ReplaceAllString(text, "")
			text = strings.TrimSpace(text)
			if len([]rune(text)) >= 4 {
				bullets = append(bullets, text)
			}
		}
	}
	if len(bullets) >= 2 {
		return bullets
	}

	parts := regexp.MustCompile(`[;；\n]+`).Split(goal, -1)
	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) >= 2 {
		return cleaned
	}
	return []string{strings.TrimSpace(goal)}
}

func parseExplicitWorkers(text string) *int {
	patterns := []string{
		`(\d+)\s*(?:个|名)?\s*(?:codex|CODEX|worker|workers|终端|并行)`,
	}
	for _, p := range patterns {
		m := regexp.MustCompile(p).FindStringSubmatch(text)
		if len(m) < 2 {
			continue
		}
		val, _ := strconv.Atoi(m[1])
		if val >= 1 && val <= 32 {
			return &val
		}
	}
	return nil
}

func estimateSameTaskWorkers(goal string) int {
	size := len(strings.TrimSpace(goal))
	workers := 5
	if size < 80 {
		workers = 4
	} else if size > 260 {
		workers = 7
	}
	heavy := []string{"重构", "架构", "跨模块", "数据库", "全链路", "端到端"}
	for _, m := range heavy {
		if strings.Contains(goal, m) {
			workers++
			break
		}
	}
	if workers < 3 {
		workers = 3
	}
	if workers > 8 {
		workers = 8
	}
	return workers
}

func decideWorkers(mode, goal string, tasks []string, explicit *int) int {
	if explicit != nil {
		return clamp(*explicit, 1, 32)
	}
	if hinted := parseExplicitWorkers(goal); hinted != nil {
		return clamp(*hinted, 1, 32)
	}
	if mode == "split-task" {
		return clamp(len(tasks), 1, 8)
	}
	return estimateSameTaskWorkers(goal)
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func detectDirectExecution(goal string) bool {
	for _, k := range directExecKeywords {
		if strings.Contains(goal, k) {
			return true
		}
	}
	return false
}

func detectAnalyzeOnly(goal string) bool {
	for _, k := range analyzeOnlyKeywords {
		if strings.Contains(goal, k) {
			return true
		}
	}
	return false
}

func detectModifyIntent(goal string) bool {
	for _, k := range modifyKeywords {
		if strings.Contains(goal, k) {
			return true
		}
	}
	return false
}

func detectExecutionKind(goal, executionKind string) string {
	if executionKind != "" && executionKind != "auto" {
		return executionKind
	}
	if detectAnalyzeOnly(goal) {
		return "analyze"
	}
	return "modify"
}

func slugify(text, fallback string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	value := strings.Trim(strings.ToLower(re.ReplaceAllString(text, "-")), "-")
	if value == "" {
		value = fallback
	}
	if len(value) > 28 {
		value = strings.Trim(value[:28], "-")
	}
	if value == "" {
		return fallback
	}
	return value
}

func defaultWorkerRow(repoRoot, runID, mode, executionKind, workerID, taskTitle, taskScope, strategy, baseBranch string) WorkerRow {
	branchSlug := slugify(workerID+"-"+choose(strategy != "-", strategy, taskTitle), workerID)
	workerBranch := "-"
	worktreePath := "."
	verifyCmd := "-"
	if executionKind == "modify" {
		workerBranch = fmt.Sprintf("orchestrator/%s/%s-%s", runID, workerID, branchSlug)
		worktreePath = filepath.Join(".worktree-tmux-orch", runID, workerID)
		verifyCmd = DefaultVerifyCmd
	}
	return WorkerRow{
		RunID:        runID,
		Mode:         mode,
		WorkerID:     workerID,
		TaskTitle:    taskTitle,
		TaskScope:    taskScope,
		Strategy:     strategy,
		BaseBranch:   baseBranch,
		WorkerBranch: workerBranch,
		WorktreePath: worktreePath,
		VerifyCmd:    verifyCmd,
		Status:       "planned",
		SessionID:    "last",
		ResultRef:    filepath.ToSlash(filepath.Join(".codex", "tools", "tmux-orch", "results", runID, workerID+".md")),
		Notes:        "execution_kind=" + executionKind,
	}
}

func choose(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func buildWorkerRows(repoRoot, runID, mode, executionKind, goal string, tasks []string, workers int, baseBranch string) []WorkerRow {
	var rows []WorkerRow
	if mode == "same-task" {
		for i := 0; i < workers; i++ {
			workerID := fmt.Sprintf("w%02d", i+1)
			strategy := sameTaskStrategies[i%len(sameTaskStrategies)]
			title := "同题多解-" + strategy
			scope := goal
			if len([]rune(scope)) > 120 {
				scope = string([]rune(scope)[:120]) + "..."
			}
			rows = append(rows, defaultWorkerRow(repoRoot, runID, mode, executionKind, workerID, title, scope, strategy, baseBranch))
		}
		return rows
	}

	bucketCount := max(1, workers)
	buckets := make([][]string, bucketCount)
	for idx, task := range tasks {
		buckets[idx%bucketCount] = append(buckets[idx%bucketCount], task)
	}
	workerIdx := 0
	for _, group := range buckets {
		if len(group) == 0 {
			continue
		}
		workerIdx++
		workerID := fmt.Sprintf("w%02d", workerIdx)
		title := group[0]
		if len([]rune(title)) > 80 {
			title = string([]rune(title)[:80])
		}
		scope := strings.Join(group, " | ")
		row := defaultWorkerRow(repoRoot, runID, mode, executionKind, workerID, title, scope, "-", baseBranch)
		row.Notes = fmt.Sprintf("subtasks=%d", len(group))
		rows = append(rows, row)
	}
	return rows
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func applyExecutionKindToRows(state *RunState) {
	runID := state.RunID
	executionKind := state.ExecutionKind
	for i := range state.Workers {
		row := &state.Workers[i]
		workerID := firstNonEmpty(row.WorkerID, "w00")
		if executionKind == "analyze" {
			row.WorkerBranch = "-"
			row.WorktreePath = "."
			row.VerifyCmd = "-"
			row.Notes = appendNote(row.Notes, "execution_kind=analyze")
			continue
		}
		if strings.TrimSpace(row.WorkerBranch) == "" || row.WorkerBranch == "-" {
			taskTitle := firstNonEmpty(row.TaskTitle, workerID)
			strategy := firstNonEmpty(row.Strategy, "-")
			branchSlug := slugify(workerID+"-"+choose(strategy != "-", strategy, taskTitle), workerID)
			row.WorkerBranch = fmt.Sprintf("orchestrator/%s/%s-%s", runID, workerID, branchSlug)
		}
		if strings.TrimSpace(row.WorktreePath) == "" || row.WorktreePath == "." || row.WorktreePath == "-" {
			row.WorktreePath = filepath.Join(".worktree-tmux-orch", runID, workerID)
		}
		if strings.TrimSpace(row.VerifyCmd) == "" || row.VerifyCmd == "-" {
			row.VerifyCmd = DefaultVerifyCmd
		}
	}
}

func appendNote(notes, message string) string {
	text := strings.TrimSpace(notes)
	if text == "" || text == "-" {
		return message
	}
	if strings.Contains(text, message) {
		return text
	}
	return text + "; " + message
}

// ----- tmux + worker runtime -----

func workerPaths(repoRoot, runID, workerID string) map[string]string {
	root := toolRoot(repoRoot)
	return map[string]string{
		"prompt":  filepath.Join(root, "state", runID, workerID+".prompt.txt"),
		"script":  filepath.Join(root, "state", runID, workerID+".run.sh"),
		"log":     filepath.Join(root, "logs", runID, workerID+".log"),
		"done":    filepath.Join(root, "logs", runID, workerID+".done"),
		"message": filepath.Join(root, "results", runID, workerID+".md"),
	}
}

func workerPrompt(state *RunState, row *WorkerRow) string {
	extra := ""
	if state.Mode == "same-task" {
		extra = "你处于同题多解模式。优先执行你负责的策略，并在保证正确性的前提下突出该策略优势。\n策略: " + row.Strategy
	} else {
		extra = "你处于任务并行模式，仅处理你负责的 task_scope。"
	}
	actionConstraints := ""
	summaryHint := ""
	if state.ExecutionKind == "analyze" {
		actionConstraints = strings.TrimSpace(`
约束:
1) 严禁修改任何代码、文档、配置或 git 历史。
2) 仅允许读取/分析命令，不执行写入类命令。
3) 输出最终总结，必须包含: 发现的问题、优先级、建议方案、风险评估。
4) 若需要修改建议，使用“建议变更”形式描述，不要实际落盘。
5) 不要输出 Markdown 本地文件链接，不要使用 file:// URI；引用文件时只写纯文本路径。
`)
		summaryHint = strings.TrimSpace(`
最终请在回复末尾附加结构化摘要，严格使用以下格式，每个字段单行：
<<<ORCH_SUMMARY
status: done|blocked|failed
summary: 一句话总结
key_changes: 分析型任务写“无实际修改”；可用分号分隔多点
verify: 写“not_run”或实际检查结论
risks: 主要风险；可用分号分隔多点
next_steps: 建议下一步；可用分号分隔多点
>>>
`)
	} else {
		actionConstraints = strings.TrimSpace(`
约束:
1) 仅在当前分支内工作，不要切到其他分支。
2) 可修改代码与文档，并自行运行必要命令。
3) 完成后执行 verify_cmd。
4) 完成后提交变更，提交信息使用中文并包含明确 scope。
5) 输出最终总结，包含: 主要改动、验证结果、风险与后续建议。
6) 不要输出 Markdown 本地文件链接，不要使用 file:// URI；引用文件时只写纯文本路径。
`)
		summaryHint = strings.TrimSpace(`
最终请在回复末尾附加结构化摘要，严格使用以下格式，每个字段单行：
<<<ORCH_SUMMARY
status: done|blocked|failed
summary: 一句话总结
key_changes: 主要改动；可用分号分隔多点
verify: 验证结果；可用分号分隔多点
risks: 风险与后续建议；可用分号分隔多点
next_steps: 建议下一步；可用分号分隔多点
>>>
`)
	}

	verifyCmd := row.VerifyCmd
	if state.ExecutionKind != "modify" {
		verifyCmd = "-"
	}
	prompt := strings.TrimSpace(fmt.Sprintf(`
你是并行 worker %s。

全局目标:
%s

你的任务:
- task_title: %s
- task_scope: %s
- branch: %s
- verify_cmd: %s
- execution_kind: %s

%s

%s

%s
`,
		row.WorkerID,
		state.Goal,
		row.TaskTitle,
		row.TaskScope,
		row.WorkerBranch,
		verifyCmd,
		state.ExecutionKind,
		actionConstraints,
		summaryHint,
		extra,
	))
	return prompt + "\n"
}

func writeWorkerFiles(repoRoot string, state *RunState, row *WorkerRow, promptText string, useResume bool) (map[string]string, error) {
	runID := state.RunID
	workerID := row.WorkerID
	paths := workerPaths(repoRoot, runID, workerID)
	for _, k := range []string{"prompt", "script", "log", "done", "message"} {
		if err := os.MkdirAll(filepath.Dir(paths[k]), 0o755); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(paths["prompt"], []byte(promptText), 0o644); err != nil {
		return nil, err
	}

	worktreeAbs := filepath.Join(repoRoot, row.WorktreePath)
	execPrefix := "codex exec --json"
	resumePrefix := "codex exec resume --last --json"
	if state.ExecutionKind == "analyze" {
		// In some environments, Codex sandboxing may rely on seccomp/userns and prevent
		// executing even read-only shell commands. Since analyze runs already forbid
		// code modifications via prompt constraints and we do not create worktrees/branches,
		// prefer bypassing sandbox to keep read-only analysis workable.
		execPrefix = "codex exec --json --dangerously-bypass-approvals-and-sandbox"
		resumePrefix = "codex exec resume --last --json --dangerously-bypass-approvals-and-sandbox"
	}
	// Keep Codex stdout/stderr attached to the tmux pane (TTY). Log collection is handled
	// by tmux `pipe-pane` to avoid Node stdio/tty reset edge cases when redirecting.
	cmdStart := execPrefix + ` -o "$MSG_FILE" "$PROMPT_TEXT"`
	cmdResume := resumePrefix + ` -o "$MSG_FILE" "$PROMPT_TEXT" || ` + execPrefix + ` -o "$MSG_FILE" "$PROMPT_TEXT"`
	cmd := cmdStart
	if useResume {
		cmd = cmdResume
	}

	script := strings.TrimSpace(fmt.Sprintf(`#!/usr/bin/env bash
set -u
WORKTREE=%s
PROMPT_FILE=%s
LOG_FILE=%s
DONE_FILE=%s
MSG_FILE=%s

mkdir -p "$(dirname "$LOG_FILE")" "$(dirname "$DONE_FILE")" "$(dirname "$MSG_FILE")"
rm -f "$DONE_FILE"
done_written=0

write_done() {
  local code="$1"
  if [ "$done_written" -eq 1 ]; then
    return
  fi
  echo "$code" >"$DONE_FILE"
  done_written=1
}

handle_interrupt() {
  write_done 130
  exit 0
}

trap 'write_done $?' EXIT
trap handle_interrupt INT TERM HUP

cd "$WORKTREE"
PROMPT_TEXT="$(cat "$PROMPT_FILE")"

if %s; then
  rc=0
else
  rc=$?
fi

write_done "$rc"
exit 0
`, shellQuote(worktreeAbs), shellQuote(paths["prompt"]), shellQuote(paths["log"]), shellQuote(paths["done"]), shellQuote(paths["message"]), cmd)) + "\n"

	if err := os.WriteFile(paths["script"], []byte(script), 0o755); err != nil {
		return nil, err
	}
	return paths, nil
}

func shellQuote(s string) string {
	// Minimal safe quoting for bash.
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func startWorker(repoRoot string, state *RunState, row *WorkerRow, paneID string, promptText string, useResume bool) error {
	paths, err := writeWorkerFiles(repoRoot, state, row, promptText, useResume)
	if err != nil {
		return err
	}
	if err := tmuxPipePane(paneID, paths["log"]); err != nil {
		// Do not fail the run just because log piping is unavailable.
		row.Notes = appendNote(row.Notes, "pipe_pane_error="+shortError(err))
	}
	// Use respawn-pane instead of send-keys so commands are not executed via the user's
	// interactive shell (e.g. zsh), avoiding polluting shell history/completions.
	if err := tmuxRespawnPane(paneID, "bash "+shellQuote(paths["script"])); err != nil {
		return err
	}
	row.PaneID = paneID
	row.Status = "running"
	row.SessionID = "last"
	row.ResultRef = filepath.ToSlash(relToRepo(repoRoot, paths["message"]))
	row.PromptFile = paths["prompt"]
	row.ScriptFile = paths["script"]
	row.LogFile = paths["log"]
	row.DoneFile = paths["done"]
	row.Notes = appendNote(row.Notes, "pane="+paneID)
	return nil
}

func shortError(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	if len(s) <= 120 {
		return s
	}
	return s[:117] + "..."
}

func relToRepo(repoRoot, path string) string {
	if repoRoot == "" {
		return path
	}
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}
	return rel
}

func rowByWorkerID(state *RunState, workerID string) (*WorkerRow, error) {
	for i := range state.Workers {
		if state.Workers[i].WorkerID == workerID {
			return &state.Workers[i], nil
		}
	}
	return nil, fmt.Errorf("worker not found: %s", workerID)
}

func workerStatusCounter(rows []WorkerRow) map[string]int {
	out := make(map[string]int)
	for _, r := range rows {
		out[r.Status]++
	}
	return out
}

func refreshWorkerStatuses(state *RunState) {
	sessionName := strings.TrimSpace(state.SessionName)
	sessionAlive := sessionName != "" && tmuxHasSession(sessionName)
	for i := range state.Workers {
		row := &state.Workers[i]
		if row.DoneFile != "" {
			if b, err := os.ReadFile(row.DoneFile); err == nil {
				codeTxt := strings.TrimSpace(string(b))
				if codeTxt != "" {
					code, _ := strconv.Atoi(codeTxt)
					row.Status = workerExitStatus(code)
					row.Notes = appendNote(row.Notes, fmt.Sprintf("exit=%d", code))
					continue
				}
			}
		}

		if row.Status == "running" {
			if sessionAlive && row.PaneID != "" && tmuxPaneExists(row.PaneID) {
				row.Status = "running"
			} else {
				row.Status = "blocked"
				row.Notes = appendNote(row.Notes, "worker_interrupted")
			}
		}
	}

	allTerminal := true
	for _, r := range state.Workers {
		if !workerIsTerminal(r.Status) {
			allTerminal = false
			break
		}
	}
	if allTerminal && sessionAlive {
		_ = tmuxKillSession(sessionName)
	}
}

func workerIsTerminal(status string) bool {
	return status == "done" || status == "failed" || status == "blocked"
}

func workerExitStatus(code int) string {
	if code == 0 {
		return "done"
	}
	if code == 130 {
		return "blocked"
	}
	return "failed"
}

func ensureWorkerPane(state *RunState, row *WorkerRow) (string, error) {
	session := strings.TrimSpace(state.SessionName)
	if session != "" && tmuxHasSession(session) && row.PaneID != "" && tmuxPaneExists(row.PaneID) {
		return row.PaneID, nil
	}
	if session == "" {
		session = "orch-" + state.RunID
		if len(session) > 40 {
			session = session[:40]
		}
		state.SessionName = session
	}
	if !tmuxHasSession(session) {
		p, err := tmuxNewSession(session)
		if err != nil {
			return "", err
		}
		row.PaneID = p
		return p, nil
	}
	p, err := tmuxNewPane(session)
	if err != nil {
		return "", err
	}
	row.PaneID = p
	return p, nil
}

// ----- git worktree -----

func ensureWorkerWorktree(repoRoot string, row *WorkerRow) error {
	baseBranch := strings.TrimSpace(row.BaseBranch)
	workerBranch := strings.TrimSpace(row.WorkerBranch)
	if baseBranch == "" {
		return fmt.Errorf("base branch is empty")
	}
	if workerBranch == "" || workerBranch == "-" {
		return fmt.Errorf("worker branch is empty")
	}

	wtPath := filepath.Join(repoRoot, row.WorktreePath)
	if st, err := os.Stat(wtPath); err == nil && st.IsDir() {
		if _, err := os.Stat(filepath.Join(wtPath, ".git")); err != nil {
			return fmt.Errorf("worktree path exists but is not git worktree: %s", wtPath)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return err
	}

	if branchExists(repoRoot, workerBranch) {
		_, err := sh(repoRoot, "git", "worktree", "add", wtPath, workerBranch)
		return err
	}
	_, err := sh(repoRoot, "git", "worktree", "add", "-b", workerBranch, wtPath, baseBranch)
	return err
}

func branchExists(repoRoot, branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoRoot
	err := cmd.Run()
	return err == nil
}

// ----- tmux helpers -----

func tmuxHasSession(session string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", session)
	err := cmd.Run()
	return err == nil
}

func tmuxNewSession(session string) (string, error) {
	// Avoid user's login shell (often zsh) for faster pane startup.
	// We run bash without rc/profile to minimize overhead.
	if _, err := sh("", "tmux", "new-session", "-d", "-s", session, "-n", "workers", "bash --noprofile --norc"); err != nil {
		return "", err
	}
	// Keep panes visible after the worker command exits (useful for capture-pane/status).
	_, _ = sh("", "tmux", "set-window-option", "-t", session+":0", "remain-on-exit", "on")
	out, err := sh("", "tmux", "display-message", "-p", "-t", session+":0.0", "#{pane_id}")
	if err != nil {
		return "", err
	}
	paneID := strings.TrimSpace(out)
	if paneID == "" {
		return "", fmt.Errorf("failed to create tmux pane")
	}
	return paneID, nil
}

func tmuxNewPane(session string) (string, error) {
	// Spawn a lightweight shell in the pane (we respawn it with the real worker command later).
	out, err := sh("", "tmux", "split-window", "-d", "-t", session+":0", "-P", "-F", "#{pane_id}", "bash --noprofile --norc")
	if err != nil {
		return "", err
	}
	_, _ = sh("", "tmux", "select-layout", "-t", session+":0", "tiled")
	return strings.TrimSpace(out), nil
}

func tmuxPaneExists(paneID string) bool {
	if strings.TrimSpace(paneID) == "" {
		return false
	}
	out, err := sh("", "tmux", "list-panes", "-a", "-F", "#{pane_id}")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == paneID {
			return true
		}
	}
	return false
}

func tmuxSend(paneID, command string) error {
	_, err := sh("", "tmux", "send-keys", "-t", paneID, command, "C-m")
	return err
}

func tmuxRespawnPane(paneID, command string) error {
	_, err := sh("", "tmux", "respawn-pane", "-k", "-t", paneID, command)
	return err
}

func tmuxPipePane(paneID, logFile string) error {
	paneID = strings.TrimSpace(paneID)
	logFile = strings.TrimSpace(logFile)
	if paneID == "" || logFile == "" {
		return nil
	}
	// Keep worker stdio as TTY; collect logs via tmux pipe-pane.
	cmd := fmt.Sprintf("cat >> %s", shellQuote(logFile))
	_, err := sh("", "tmux", "pipe-pane", "-t", paneID, cmd)
	return err
}

func tmuxCtrlC(paneID string) error {
	_, err := sh("", "tmux", "send-keys", "-t", paneID, "C-c")
	return err
}

func tmuxKillSession(session string) error {
	_, err := sh("", "tmux", "kill-session", "-t", session)
	return err
}

// ----- git helpers -----

func sh(dir string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		s := strings.TrimSpace(stderr.String())
		if s == "" {
			s = strings.TrimSpace(stdout.String())
		}
		if s == "" {
			s = err.Error()
		}
		return "", fmt.Errorf("%s: %s", strings.Join(args, " "), s)
	}
	return stdout.String(), nil
}

func gitRepoRoot() (string, error) {
	out, err := sh("", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitCurrentBranch(repoRoot string) (string, error) {
	out, err := sh(repoRoot, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
	if branch == "HEAD" {
		return "", errors.New("detached HEAD is not supported")
	}
	return branch, nil
}

// ----- misc -----

func resizeSameTaskWorkers(state *RunState, newCount int) {
	newCount = clamp(newCount, 1, 32)
	baseBranch := state.BaseBranch
	runID := state.RunID

	fresh := buildWorkerRows("", runID, "same-task", state.ExecutionKind, state.Goal, []string{state.Goal}, newCount, baseBranch)
	oldByID := make(map[string]WorkerRow)
	for _, r := range state.Workers {
		oldByID[r.WorkerID] = r
	}
	var merged []WorkerRow
	for _, r := range fresh {
		if old, ok := oldByID[r.WorkerID]; ok {
			// Keep runtime fields.
			r.Status = old.Status
			r.SessionID = old.SessionID
			r.ResultRef = old.ResultRef
			r.Notes = old.Notes
			r.PaneID = old.PaneID
			r.PromptFile = old.PromptFile
			r.ScriptFile = old.ScriptFile
			r.LogFile = old.LogFile
			r.DoneFile = old.DoneFile
		}
		merged = append(merged, r)
	}
	state.Workers = merged
}

// Defensive: prevent panics in the MCP server.
func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

func init() {
	// Ensure deterministic behavior isn't required here.
	_ = rand.Uint32()
}
