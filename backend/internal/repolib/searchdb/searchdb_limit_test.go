package searchdb

import "testing"

func TestStableRecallLimitKeepsSmallQueriesOnSameCandidateWindow(t *testing.T) {
	if got := stableRecallLimit(3); got != 20 {
		t.Fatalf("expected recall limit 20 for topK=3, got %d", got)
	}
	if got := stableRecallLimit(8); got != 20 {
		t.Fatalf("expected recall limit 20 for topK=8, got %d", got)
	}
	if got := stableRecallLimit(24); got != 24 {
		t.Fatalf("expected recall limit 24 for topK=24, got %d", got)
	}
}
