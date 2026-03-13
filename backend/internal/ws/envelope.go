package ws

// Envelope 功能：统一 WS 事件封装（type/ts + workflow/node/execution 上下文 + payload）。
// 参数/返回：作为 json 序列化载体使用；无直接行为。
// 失败场景：无（纯数据结构）。
// 副作用：无。
type Envelope struct {
	Type            string      `json:"type"`
	Ts              int64       `json:"ts"`
	WorkflowID      string      `json:"workflow_id,omitempty"`
	NodeID          string      `json:"node_id,omitempty"`
	OrchestrationID string      `json:"orchestration_id,omitempty"`
	RoundID         string      `json:"round_id,omitempty"`
	AgentRunID      string      `json:"agent_run_id,omitempty"`
	ExecutionID     string      `json:"execution_id,omitempty"`
	Payload         interface{} `json:"payload,omitempty"`
}
