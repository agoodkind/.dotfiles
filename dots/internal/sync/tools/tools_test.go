package tools

import (
	"context"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/sync/common"
)

// TestGetLatestCrateVersionAsyncCmd exercises the real crates.io endpoint end to end.
// crates.io returns HTTP 403 to requests without a User-Agent header, so a missing or
// empty header surfaces here as a "crates api error 403" and fails the test. A transport
// error (no network) skips instead, so the test stays meaningful offline without mocking.
func TestGetLatestCrateVersionAsyncCmd(t *testing.T) {
	version, err := getLatestCrateVersion(context.Background(), "async-cmd")
	if err != nil {
		if strings.Contains(err.Error(), "crates api error") {
			t.Fatalf("crates.io rejected the request: %v", err)
		}
		t.Skipf("crates.io unreachable, skipping: %v", err)
	}
	if version == "" {
		t.Fatal("getLatestCrateVersion returned an empty version")
	}
	if !common.VersionAtLeast(version, "0.1.1") {
		t.Fatalf("getLatestCrateVersion returned %q, want >= 0.1.1", version)
	}
}
