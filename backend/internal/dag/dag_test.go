package dag

import "testing"

func TestExtractFirstJSONObject(t *testing.T) {
	t.Parallel()

	raw := []byte("hello world\n\n  {\"k\":1}\nbye\n")
	obj, err := ExtractFirstJSONObject(raw)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if string(obj) != `{"k":1}` {
		t.Fatalf("unexpected obj: %q", string(obj))
	}
}

func TestValidateAcceptsValidDAG(t *testing.T) {
	t.Parallel()

	d := DAG{
		Nodes: []DAGNode{
			{ID: "a", ExpertID: "bash", Prompt: "echo hi"},
		},
		Edges: nil,
	}
	if err := Validate(d, ValidateOptions{KnownExperts: map[string]struct{}{"bash": {}}}); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateRejectsDuplicateNodeID(t *testing.T) {
	t.Parallel()

	d := DAG{
		Nodes: []DAGNode{
			{ID: "a", ExpertID: "bash", Prompt: "echo 1"},
			{ID: "a", ExpertID: "bash", Prompt: "echo 2"},
		},
	}
	if err := Validate(d, ValidateOptions{KnownExperts: map[string]struct{}{"bash": {}}}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateRejectsUnknownEdgeReference(t *testing.T) {
	t.Parallel()

	d := DAG{
		Nodes: []DAGNode{
			{ID: "a", ExpertID: "bash", Prompt: "echo a"},
		},
		Edges: []DAGEdge{
			{From: "a", To: "b", Type: "success"},
		},
	}
	if err := Validate(d, ValidateOptions{KnownExperts: map[string]struct{}{"bash": {}}}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateRejectsCycle(t *testing.T) {
	t.Parallel()

	d := DAG{
		Nodes: []DAGNode{
			{ID: "a", ExpertID: "bash", Prompt: "echo a"},
			{ID: "b", ExpertID: "bash", Prompt: "echo b"},
		},
		Edges: []DAGEdge{
			{From: "a", To: "b", Type: "success"},
			{From: "b", To: "a", Type: "success"},
		},
	}
	if err := Validate(d, ValidateOptions{KnownExperts: map[string]struct{}{"bash": {}}}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateRejectsUnknownExpert(t *testing.T) {
	t.Parallel()

	d := DAG{
		Nodes: []DAGNode{
			{ID: "a", ExpertID: "claudecode", Prompt: "echo hi"},
		},
	}
	if err := Validate(d, ValidateOptions{KnownExperts: map[string]struct{}{"bash": {}}}); err == nil {
		t.Fatalf("expected error")
	}
}
