package platform

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMacCaskAppPathsChecksAppBundleSuffix(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "tmp", "home")

	got := macCaskAppPaths("Visual Studio Code", home)
	want := []string{
		filepath.Join(string(filepath.Separator), "Applications", "Visual Studio Code"),
		filepath.Join(string(filepath.Separator), "Applications", "Visual Studio Code.app"),
		filepath.Join(home, "Applications", "Visual Studio Code"),
		filepath.Join(home, "Applications", "Visual Studio Code.app"),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("macCaskAppPaths() = %#v, want %#v", got, want)
	}
}

func TestMacCaskAppPathsDoesNotDuplicateAppSuffix(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "tmp", "home")

	got := macCaskAppPaths("Ghostty.app", home)
	want := []string{
		filepath.Join(string(filepath.Separator), "Applications", "Ghostty.app"),
		filepath.Join(home, "Applications", "Ghostty.app"),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("macCaskAppPaths() = %#v, want %#v", got, want)
	}
}

func TestMacCaskAppExistsFindsHomeAppBundle(t *testing.T) {
	home := t.TempDir()
	appPath := filepath.Join(home, "Applications", "Ghostty.app")
	if err := os.MkdirAll(appPath, 0o755); err != nil {
		t.Fatalf("creating app bundle: %v", err)
	}

	if !macCaskAppExists("Ghostty", home) {
		t.Fatal("macCaskAppExists() = false, want true")
	}
}
