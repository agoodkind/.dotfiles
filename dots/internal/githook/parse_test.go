package githook

import (
	"strings"
	"testing"
)

func TestParseRefUpdatesReadsHeadSymbolicLine(t *testing.T) {
	input := "0000000000000000000000000000000000000000 ref:refs/heads/feature HEAD\n"
	updates, err := readRefUpdates(t.Context(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("readRefUpdates returned error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("len(updates) = %d, want 1", len(updates))
	}
	if updates[0].refName != "HEAD" {
		t.Fatalf("refName = %q, want HEAD", updates[0].refName)
	}
	if updates[0].newValue != "ref:refs/heads/feature" {
		t.Fatalf("newValue = %q, want ref:refs/heads/feature", updates[0].newValue)
	}
	branch, ok := branchFromSymbolicValue(updates[0].newValue)
	if !ok {
		t.Fatal("branchFromSymbolicValue returned false, want true")
	}
	if branch != "feature" {
		t.Fatalf("branch = %q, want feature", branch)
	}
}

func TestParseRefUpdatesRejectsMalformedLine(t *testing.T) {
	_, err := readRefUpdates(t.Context(), strings.NewReader("only-one-field\n"))
	if err == nil {
		t.Fatal("readRefUpdates returned nil error, want malformed-line error")
	}
}
