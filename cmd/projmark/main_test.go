package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return string(outBytes), string(errBytes)
}

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
	_, stderr := captureOutput(t, func() {
		code := run([]string{})
		if code != 1 {
			t.Errorf("expected exit code 1 for no args, got %d", code)
		}
	})
	if !strings.Contains(stderr, "Usage:") {
		t.Error("expected usage text in stderr for no args")
	}
	if !strings.Contains(stderr, "Examples:") {
		t.Error("expected examples section in stderr for no args")
	}
}

func TestRun_HelpFlag(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		code := run([]string{"-h"})
		if code != 0 {
			t.Errorf("expected exit code 0 for -h, got %d", code)
		}
	})
	if !strings.Contains(stderr, "Usage:") {
		t.Error("expected usage text in stderr for -h")
	}
}

func TestRun_InvalidFlag(t *testing.T) {
	code := run([]string{"--bogus"})
	if code != 1 {
		t.Errorf("expected exit code 1 for invalid flag, got %d", code)
	}
}

func TestRun_InvalidDirectory(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		code := run([]string{"/nonexistent/path"})
		if code != 1 {
			t.Errorf("expected exit code 1 for invalid dir, got %d", code)
		}
	})
	if !strings.Contains(stderr, "does not exist") {
		t.Errorf("expected 'does not exist' in stderr, got: %s", stderr)
	}
}

func TestRun_FileNotDirectory(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "afile.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr := captureOutput(t, func() {
		code := run([]string{filePath})
		if code != 1 {
			t.Errorf("expected exit code 1 for file path, got %d", code)
		}
	})
	if !strings.Contains(stderr, "is not a directory") {
		t.Errorf("expected 'is not a directory' in stderr, got: %s", stderr)
	}
}

func TestRun_BrokenSymlink(t *testing.T) {
	tmp := t.TempDir()
	linkPath := filepath.Join(tmp, "broken-link")
	if err := os.Symlink("/nonexistent/target", linkPath); err != nil {
		t.Fatal(err)
	}

	_, stderr := captureOutput(t, func() {
		code := run([]string{linkPath})
		if code != 1 {
			t.Errorf("expected exit code 1 for broken symlink, got %d", code)
		}
	})
	if !strings.Contains(stderr, "symlink to a non-existent target") {
		t.Errorf("expected symlink error in stderr, got: %s", stderr)
	}
}

func TestRun_Version(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--version"})
		if code != 0 {
			t.Errorf("expected exit code 0 for --version, got %d", code)
		}
	})
	if !strings.Contains(stdout, ProgramName) {
		t.Errorf("expected program name in version output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "build:") {
		t.Errorf("expected build info in version output, got: %s", stdout)
	}
}

func TestRun_ScanWorkspace(t *testing.T) {
	root := setupMockWorkspace(t)

	stdout, _ := captureOutput(t, func() {
		code := run([]string{root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stdout, "Scanning:") {
		t.Error("expected 'Scanning:' in output")
	}
	if !strings.Contains(stdout, "Complete!") {
		t.Error("expected 'Complete!' in output")
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
	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--completion-bash"})
		if code != 0 {
			t.Errorf("expected exit code 0 for --completion-bash, got %d", code)
		}
	})
	if !strings.Contains(stdout, "_projmark") {
		t.Error("expected bash completion function in output")
	}
}

func TestRun_CompletionZsh(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--completion-zsh"})
		if code != 0 {
			t.Errorf("expected exit code 0 for --completion-zsh, got %d", code)
		}
	})
	if !strings.Contains(stdout, "#compdef") {
		t.Error("expected zsh completion header in output")
	}
}

func TestRun_CompletionFish(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--completion-fish"})
		if code != 0 {
			t.Errorf("expected exit code 0 for --completion-fish, got %d", code)
		}
	})
	if !strings.Contains(stdout, "complete -c projmark") {
		t.Error("expected fish completion commands in output")
	}
}
