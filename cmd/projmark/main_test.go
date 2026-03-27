package main

import (
	"os"
	"path/filepath"
	"testing"
)

func setupMockWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Cubase project with Mixdown (should be tagged)
	track1 := filepath.Join(root, "Music", "Track1")
	if err := os.MkdirAll(filepath.Join(track1, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(track1, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Cubase project without Mixdown (should NOT be tagged)
	track2 := filepath.Join(root, "Music", "Track2")
	if err := os.MkdirAll(track2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(track2, "Track2.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// LUNA project with Exported Files (should be tagged)
	luna := filepath.Join(root, "Music", "Session.luna")
	if err := os.MkdirAll(filepath.Join(luna, "Exported Files"), 0755); err != nil {
		t.Fatal(err)
	}

	// Regular directory (should be ignored)
	if err := os.MkdirAll(filepath.Join(root, "Documents"), 0755); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestRun_NoArgs(t *testing.T) {
	code := run([]string{})
	if code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d", code)
	}
}

func TestRun_InvalidDirectory(t *testing.T) {
	code := run([]string{"/nonexistent/path"})
	if code != 1 {
		t.Errorf("expected exit code 1 for invalid dir, got %d", code)
	}
}

func TestRun_Version(t *testing.T) {
	code := run([]string{"--version"})
	if code != 0 {
		t.Errorf("expected exit code 0 for --version, got %d", code)
	}
}

func TestRun_ScanWorkspace(t *testing.T) {
	root := setupMockWorkspace(t)

	code := run([]string{root})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestRun_RemoveMode(t *testing.T) {
	root := setupMockWorkspace(t)

	// First tag
	code := run([]string{root})
	if code != 0 {
		t.Fatalf("expected exit code 0 for tagging, got %d", code)
	}

	// Then remove
	code = run([]string{"-r", root})
	if code != 0 {
		t.Errorf("expected exit code 0 for untagging, got %d", code)
	}
}

func TestRun_MultipleDirectories(t *testing.T) {
	root1 := setupMockWorkspace(t)
	root2 := setupMockWorkspace(t)

	code := run([]string{root1, root2})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestRun_CompletionBash(t *testing.T) {
	code := run([]string{"--completion-bash"})
	if code != 0 {
		t.Errorf("expected exit code 0 for --completion-bash, got %d", code)
	}
}

func TestRun_CompletionZsh(t *testing.T) {
	code := run([]string{"--completion-zsh"})
	if code != 0 {
		t.Errorf("expected exit code 0 for --completion-zsh, got %d", code)
	}
}

func TestRun_CompletionFish(t *testing.T) {
	code := run([]string{"--completion-fish"})
	if code != 0 {
		t.Errorf("expected exit code 0 for --completion-fish, got %d", code)
	}
}
