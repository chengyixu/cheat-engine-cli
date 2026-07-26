package feedback

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIssueLifecycle(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	store.clock = func() time.Time { return time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC) }
	issue, err := store.Create("bug", "scan output was truncated", "0.1.0", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.Transition(issue.ID, "resolved")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("status = %q", updated.Status)
	}
	issues, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d", len(issues))
	}
}
