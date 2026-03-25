package vectorindex

import "testing"

func TestPlanBoundaryRerankCandidatesKeepsOnlyBoundaryWindow(t *testing.T) {
	approx := []*Result{
		{ID: 1, Distance: 1.00, ErrorBound: 0.02},
		{ID: 2, Distance: 1.05, ErrorBound: 0.02},
		{ID: 3, Distance: 1.10, ErrorBound: 0.02},
		{ID: 4, Distance: 1.12, ErrorBound: 0.03},
		{ID: 5, Distance: 1.30, ErrorBound: 0.01},
	}

	definiteIn, ambiguous := planBoundaryRerankCandidates(approx, 3)
	if len(definiteIn) != 2 {
		t.Fatalf("expected 2 definite-in results, got %d", len(definiteIn))
	}
	if definiteIn[0].ID != 1 || definiteIn[1].ID != 2 {
		t.Fatalf("unexpected definite-in IDs: %d, %d", definiteIn[0].ID, definiteIn[1].ID)
	}
	if len(ambiguous) != 2 {
		t.Fatalf("expected 2 ambiguous results, got %d", len(ambiguous))
	}
	if ambiguous[0].ID != 3 || ambiguous[1].ID != 4 {
		t.Fatalf("unexpected ambiguous IDs: %d, %d", ambiguous[0].ID, ambiguous[1].ID)
	}
}
