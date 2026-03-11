package tmuxorch

type DraftRequest struct {
	Goal          string `json:"goal"`
	Mode          string `json:"mode,omitempty"`           // auto|same-task|split-task
	ExecutionKind string `json:"execution_kind,omitempty"` // auto|modify|analyze
	Workers       *int   `json:"workers,omitempty"`
	RunID         string `json:"run_id,omitempty"`
}

type ReviseRequest struct {
	RunID    string `json:"run_id"`
	Feedback string `json:"feedback"`
}

type RunRequest struct {
	RunID           string `json:"run_id"`
	ReuseSession    bool   `json:"reuse_session,omitempty"`
	LaunchStaggerMs *int   `json:"launch_stagger_ms,omitempty"`
}

type ControlRequest struct {
	RunID    string `json:"run_id"`
	WorkerID string `json:"worker_id"`
	Action   string `json:"action"`           // stop|inject|restart
	Prompt   string `json:"prompt,omitempty"` // required for inject
}

type StatusRequest struct {
	RunID string `json:"run_id"`

	IncludePane     *bool `json:"include_pane,omitempty"`
	IncludePaneTail *bool `json:"include_pane_tail,omitempty"`
	PaneTailLines   *int  `json:"pane_tail_lines,omitempty"`

	IncludeLogTail *bool `json:"include_log_tail,omitempty"`
	LogTailLines   *int  `json:"log_tail_lines,omitempty"`
}

type CloseRequest struct {
	RunID string `json:"run_id"`
}
