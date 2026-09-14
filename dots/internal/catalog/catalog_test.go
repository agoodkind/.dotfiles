package catalog

import (
	"testing"
)

func TestDefaultPackageConfigCopiesAptRepos(t *testing.T) {
	t.Parallel()

	cfg := DefaultPackageConfig()
	if len(cfg.AptRepos) == 0 {
		t.Fatal("AptRepos is empty, want ookla-speedtest from packages.toml")
	}
	if cfg.AptRepos[0].ID != "ookla-speedtest" {
		t.Fatalf("AptRepos[0].ID = %q, want ookla-speedtest", cfg.AptRepos[0].ID)
	}
	if !containsString(cfg.BrewSpecific, "teamookla/speedtest/speedtest") {
		t.Fatal("BrewSpecific missing teamookla/speedtest/speedtest")
	}
	if containsString(cfg.AptSpecific, "speedtest-cli") {
		t.Fatal("AptSpecific still lists speedtest-cli")
	}
	if !containsString(cfg.AptSpecific, "speedtest") {
		t.Fatal("AptSpecific missing speedtest")
	}

	cfg.AptRepos[0].ID = "mutated"
	copied := DefaultPackageConfig()
	if copied.AptRepos[0].ID == "mutated" {
		t.Fatal("DefaultPackageConfig() did not copy AptRepos")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
