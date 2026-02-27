package dag

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type DAG struct {
	WorkflowTitle string    `json:"workflow_title"`
	Nodes         []DAGNode `json:"nodes"`
	Edges         []DAGEdge `json:"edges"`
}

type DAGNode struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Type             string  `json:"type"`
	ExpertID         string  `json:"expert_id"`
	FallbackExpertID string  `json:"fallback_expert_id"`
	Complexity       string  `json:"complexity"`
	QualityTier      string  `json:"quality_tier"`
	Model            *string `json:"model"`
	RoutingReason    string  `json:"routing_reason"`
	Prompt           string  `json:"prompt"`
}

type DAGEdge struct {
	From         string  `json:"from"`
	To           string  `json:"to"`
	Type         string  `json:"type"`
	SourceHandle *string `json:"source_handle"`
	TargetHandle *string `json:"target_handle"`
}

type ValidateOptions struct {
	KnownExperts map[string]struct{}
}

// ExtractFirstJSONObject 功能：从包含杂文本的输出中提取第一个 JSON 对象（允许 JSON 前后有解释文本）。
// 参数/返回：b 为原始 stdout/stderr 合并输出；返回该 JSON 对象的 RawMessage。
// 失败场景：未找到可解析的 JSON 对象或解析失败时返回 error。
// 副作用：无（纯解析）。
func ExtractFirstJSONObject(b []byte) (json.RawMessage, error) {
	for i := 0; i < len(b); i++ {
		if b[i] != '{' {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(b[i:]))
		dec.UseNumber()
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			continue
		}
		if len(raw) == 0 || raw[0] != '{' {
			continue
		}
		return raw, nil
	}
	return nil, fmt.Errorf("no json object found")
}

// ParseAndValidate 功能：从输出中提取 DAG JSON 并解析校验（MVP：无环、引用存在、节点唯一、expert_id 可选校验）。
// 参数/返回：output 为原始输出；opts 可选提供 KnownExperts；返回 DAG 与错误信息。
// 失败场景：JSON 提取/解析失败、校验不通过时返回 error。
// 副作用：无（纯解析/校验）。
func ParseAndValidate(output []byte, opts ValidateOptions) (DAG, error) {
	raw, err := ExtractFirstJSONObject(output)
	if err != nil {
		return DAG{}, err
	}

	var d DAG
	if err := json.Unmarshal(raw, &d); err != nil {
		return DAG{}, fmt.Errorf("parse dag json: %w", err)
	}
	if err := Validate(d, opts); err != nil {
		return DAG{}, err
	}
	return d, nil
}

// Validate 功能：校验 DAG 是否满足 MVP 约束：无环、node.id 唯一、edges 引用的节点存在、expert_id 合法（如提供）。
// 参数/返回：d 为 DAG；opts 可选提供 KnownExperts；通过返回 nil。
// 失败场景：任一约束不满足时返回 error（可直接展示给用户）。
// 副作用：无。
func Validate(d DAG, opts ValidateOptions) error {
	if len(d.Nodes) == 0 {
		return fmt.Errorf("dag.nodes is required")
	}

	nodeIndex := make(map[string]int, len(d.Nodes))
	for i, n := range d.Nodes {
		if n.ID == "" {
			return fmt.Errorf("dag.nodes[%d].id is required", i)
		}
		if _, exists := nodeIndex[n.ID]; exists {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		nodeIndex[n.ID] = i
		if n.Prompt == "" {
			return fmt.Errorf("dag.nodes[%d].prompt is required", i)
		}
		if n.ExpertID == "" {
			return fmt.Errorf("dag.nodes[%d].expert_id is required", i)
		}
		if opts.KnownExperts != nil {
			if _, ok := opts.KnownExperts[n.ExpertID]; !ok {
				return fmt.Errorf("unknown expert_id %q (node %q)", n.ExpertID, n.ID)
			}
		}
	}

	adj := make(map[string][]string, len(d.Nodes))
	indeg := make(map[string]int, len(d.Nodes))
	for _, n := range d.Nodes {
		indeg[n.ID] = 0
	}

	for i, e := range d.Edges {
		if e.From == "" || e.To == "" {
			return fmt.Errorf("dag.edges[%d].from/to is required", i)
		}
		if _, ok := nodeIndex[e.From]; !ok {
			return fmt.Errorf("dag.edges[%d].from references unknown node %q", i, e.From)
		}
		if _, ok := nodeIndex[e.To]; !ok {
			return fmt.Errorf("dag.edges[%d].to references unknown node %q", i, e.To)
		}
		if e.From == e.To {
			return fmt.Errorf("dag.edges[%d] creates self-cycle for node %q", i, e.From)
		}
		adj[e.From] = append(adj[e.From], e.To)
		indeg[e.To]++
	}

	// Kahn: detect cycles.
	queue := make([]string, 0, len(d.Nodes))
	for id, v := range indeg {
		if v == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, to := range adj[id] {
			indeg[to]--
			if indeg[to] == 0 {
				queue = append(queue, to)
			}
		}
	}
	if visited != len(d.Nodes) {
		return fmt.Errorf("dag has cycle")
	}
	return nil
}
