package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sha1n/project-marker/internal/config"
	"github.com/sha1n/project-marker/internal/engine"
	"github.com/sha1n/project-marker/internal/scanner"
)

// captureOutput redirects os.Stdout and os.Stderr for the duration of fn.
// This mutates global state — tests in this package must NOT use t.Parallel().
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
	session := filepath.Join(root, "Music", "My Session")
	if err := os.MkdirAll(filepath.Join(session, "My Session.luna"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(session, "Exported Files"), 0755); err != nil {
		t.Fatal(err)
	}

	// Regular directory (should be ignored)
	if err := os.MkdirAll(filepath.Join(root, "Documents"), 0755); err != nil {
		t.Fatal(err)
	}

	return root
}

// failTagger returns configured errors from Apply/Remove.
type failTagger struct {
	applyErr  error
	removeErr error
}

func (f *failTagger) Apply(_, _ string) error  { return f.applyErr }
func (f *failTagger) Remove(_, _ string) error { return f.removeErr }

// alwaysTaggedTagger records Apply/Remove calls and always reports tags as present.
type alwaysTaggedTagger struct {
	applied []string
	removed []string
}

func (m *alwaysTaggedTagger) Apply(path, _ string) error {
	m.applied = append(m.applied, path)
	return nil
}

func (m *alwaysTaggedTagger) Remove(path, _ string) error {
	m.removed = append(m.removed, path)
	return nil
}

func (m *alwaysTaggedTagger) HasTag(_, _ string) (bool, error) {
	return true, nil
}

// overrideLoadConfig replaces loadConfig for the duration of the test.
func overrideLoadConfig(t *testing.T, fn func(*engine.Registry) ([]config.ResolvedTarget, error)) {
	t.Helper()
	orig := loadConfig
	loadConfig = fn
	t.Cleanup(func() { loadConfig = orig })
}

// overrideNewTagger replaces newTagger for the duration of the test.
func overrideNewTagger(t *testing.T, tagger scanner.Tagger) {
	t.Helper()
	orig := newTagger
	newTagger = func() scanner.Tagger { return tagger }
	t.Cleanup(func() { newTagger = orig })
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

func TestRun_StatPermissionError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(blocker, "child")

	_, stderr := captureOutput(t, func() {
		code := run([]string{badPath})
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("expected 'Error:' in stderr, got: %s", stderr)
	}
	if strings.Contains(stderr, "does not exist") {
		t.Errorf("should not say 'does not exist' for ENOTDIR, got: %s", stderr)
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

	code := run([]string{root})
	if code != 0 {
		t.Fatalf("expected exit code 0 for tagging, got %d", code)
	}

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

func TestRun_VerboseFlag(t *testing.T) {
	root := setupMockWorkspace(t)

	_, stderr := captureOutput(t, func() {
		code := run([]string{"-v", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stderr, "◦") && !strings.Contains(stderr, "●") {
		t.Error("expected verbose trace symbols on stderr with -v flag")
	}
}

func TestRun_VerboseLongFlag(t *testing.T) {
	root := setupMockWorkspace(t)

	_, stderr := captureOutput(t, func() {
		code := run([]string{"--verbose", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stderr, "◦") && !strings.Contains(stderr, "●") {
		t.Error("expected verbose trace symbols on stderr with --verbose flag")
	}
}

func TestRun_DebugFlag(t *testing.T) {
	root := setupMockWorkspace(t)

	_, stderr := captureOutput(t, func() {
		code := run([]string{"--debug", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stderr, "loading configuration") {
		t.Error("expected slog debug output on stderr with --debug flag")
	}
	if !strings.Contains(stderr, "starting scan") {
		t.Error("expected 'starting scan' in debug output")
	}
}

func TestRun_ConfigLoadFailure(t *testing.T) {
	overrideLoadConfig(t, func(*engine.Registry) ([]config.ResolvedTarget, error) {
		return nil, errors.New("bad config")
	})
	root := setupMockWorkspace(t)

	_, stderr := captureOutput(t, func() {
		code := run([]string{root})
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})
	if !strings.Contains(stderr, "failed to load config") {
		t.Errorf("expected 'failed to load config' in stderr, got: %s", stderr)
	}
}

func TestRun_ScanFailure(t *testing.T) {
	root := t.TempDir()
	scanDir := filepath.Join(root, "workspace")
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		t.Fatal(err)
	}

	overrideLoadConfig(t, func(r *engine.Registry) ([]config.ResolvedTarget, error) {
		_ = os.RemoveAll(scanDir)
		return config.Load(r)
	})

	_, stderr := captureOutput(t, func() {
		code := run([]string{scanDir})
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})
	if !strings.Contains(stderr, "scan failed") {
		t.Errorf("expected 'scan failed' in stderr, got: %s", stderr)
	}
}

func TestRun_SkippedInSummary(t *testing.T) {
	root := setupMockWorkspace(t)
	overrideNewTagger(t, &failTagger{applyErr: errors.New("xattr fail")})

	stdout, _ := captureOutput(t, func() {
		code := run([]string{root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stdout, "skipped") {
		t.Errorf("expected 'skipped' in stdout, got: %s", stdout)
	}
}

func TestRun_Pluralize(t *testing.T) {
	if got := pluralize(1); got != "y" {
		t.Errorf("pluralize(1) = %q, want %q", got, "y")
	}
	if got := pluralize(0); got != "ies" {
		t.Errorf("pluralize(0) = %q, want %q", got, "ies")
	}
	if got := pluralize(5); got != "ies" {
		t.Errorf("pluralize(5) = %q, want %q", got, "ies")
	}
}

func TestRun_UntaggedOutput(t *testing.T) {
	root := setupMockWorkspace(t)

	stdout, _ := captureOutput(t, func() {
		code := run([]string{"-r", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stdout, "Untagged") {
		t.Errorf("expected 'Untagged' in summary, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Removing tags from") {
		t.Errorf("expected 'Removing tags from' in stdout, got: %s", stdout)
	}
}

func TestRun_OutputUsesRelativePaths(t *testing.T) {
	root := setupMockWorkspace(t)

	stdout, _ := captureOutput(t, func() {
		code := run([]string{root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	// The header should contain the full temp dir path
	if !strings.Contains(stdout, "Scanning: "+root) {
		t.Errorf("expected header with full root path, got: %s", stdout)
	}
	// Result lines should use relative paths (just the directory name), not the full temp dir
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "✓") || strings.Contains(line, "✗") {
			// Result lines should NOT contain the root prefix
			if strings.Contains(line, root) {
				t.Errorf("result line should use relative path, but contains root prefix: %s", line)
			}
		}
	}
	// Should contain just directory names like "Track1"
	if !strings.Contains(stdout, "Track1") {
		t.Errorf("expected 'Track1' directory name in output, got: %s", stdout)
	}
}

func TestRun_DryRunFlag(t *testing.T) {
	root := setupMockWorkspace(t)

	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--dry-run", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stdout, "Would tag") {
		t.Errorf("expected 'Would tag' in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Dry run") {
		t.Errorf("expected 'Dry run' in stdout, got: %s", stdout)
	}
}

func TestRun_MultiRootGroupedOutput(t *testing.T) {
	root1 := setupMockWorkspace(t)
	root2 := setupMockWorkspace(t)

	stdout, _ := captureOutput(t, func() {
		code := run([]string{root1, root2})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	// Each root should get its own header
	if !strings.Contains(stdout, "Scanning: "+root1) {
		t.Errorf("expected 'Scanning: %s' header, got: %s", root1, stdout)
	}
	if !strings.Contains(stdout, "Scanning: "+root2) {
		t.Errorf("expected 'Scanning: %s' header, got: %s", root2, stdout)
	}
	// There should be a blank line between the two root groups
	idx1 := strings.Index(stdout, "Scanning: "+root1)
	idx2 := strings.Index(stdout, "Scanning: "+root2)
	if idx1 >= idx2 {
		t.Fatal("expected root1 header before root2 header")
	}
	between := stdout[idx1:idx2]
	if !strings.Contains(between, "\n\n") {
		t.Errorf("expected blank line between root groups, got: %q", between)
	}
}

func TestRun_DryRunVerbose(t *testing.T) {
	root := setupMockWorkspace(t)

	_, stderr := captureOutput(t, func() {
		code := run([]string{"--dry-run", "-v", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	// Verbose handler should emit open circle for dry-run would_tag events
	if !strings.Contains(stderr, "○") {
		t.Errorf("expected open circle in verbose dry-run output, got: %s", stderr)
	}
}

func TestFindRoot(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		roots []string
		want  string
	}{
		{
			name:  "path under root",
			path:  "/a/b/c",
			roots: []string{"/a/b"},
			want:  "/a/b",
		},
		{
			name:  "path equals root",
			path:  "/a/b",
			roots: []string{"/a/b"},
			want:  "/a/b",
		},
		{
			name:  "path not under any root",
			path:  "/x/y/z",
			roots: []string{"/a/b"},
			want:  "",
		},
		{
			name:  "trailing slash on root",
			path:  "/a/b/c",
			roots: []string{"/a/b/"},
			want:  "/a/b",
		},
		{
			name:  "nested roots returns most specific",
			path:  "/a/b/c/d",
			roots: []string{"/a/b", "/a/b/c"},
			want:  "/a/b/c",
		},
		{
			name:  "nested roots reversed order",
			path:  "/a/b/c/d",
			roots: []string{"/a/b/c", "/a/b"},
			want:  "/a/b/c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findRoot(tt.path, tt.roots)
			if got != tt.want {
				t.Errorf("findRoot(%q, %v) = %q, want %q", tt.path, tt.roots, got, tt.want)
			}
		})
	}
}

func TestRun_EmptyResultsForOneRoot(t *testing.T) {
	root1 := setupMockWorkspace(t)
	root2 := t.TempDir() // empty — no projects to match

	stdout, _ := captureOutput(t, func() {
		code := run([]string{root1, root2})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	// Both roots should get headers
	if !strings.Contains(stdout, "Scanning: "+root1) {
		t.Errorf("expected header for root1, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Scanning: "+root2) {
		t.Errorf("expected header for root2, got: %s", stdout)
	}
}

func TestRun_DryRunRemoveMode(t *testing.T) {
	root := setupMockWorkspace(t)

	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--dry-run", "-r", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stdout, "Dry run") {
		t.Errorf("expected 'Dry run' in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Removing tags from") {
		t.Errorf("expected 'Removing tags from' in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Would untag") {
		t.Errorf("expected 'Would untag' in summary, got: %s", stdout)
	}
}

func TestRun_DryRunWouldUntagOutput(t *testing.T) {
	root := setupMockWorkspace(t)

	overrideNewTagger(t, &alwaysTaggedTagger{})

	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--dry-run", "-r", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stdout, "Would untag") {
		t.Errorf("expected 'Would untag' result line in stdout, got: %s", stdout)
	}
}

func TestRun_DryRunAlreadyTaggedOutput(t *testing.T) {
	root := setupMockWorkspace(t)

	overrideNewTagger(t, &alwaysTaggedTagger{})

	stdout, _ := captureOutput(t, func() {
		code := run([]string{"--dry-run", root})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(stdout, "Already tagged") {
		t.Errorf("expected 'Already tagged' result line in stdout, got: %s", stdout)
	}
}

func TestRun_CompletionScriptsContainAllFlags(t *testing.T) {
	requiredFlags := []string{"-h", "-r", "--version", "--verbose", "--debug",
		"--dry-run", "--completion-bash", "--completion-zsh", "--completion-fish"}

	tests := []struct {
		name string
		arg  string
	}{
		{"bash", "--completion-bash"},
		{"zsh", "--completion-zsh"},
		{"fish", "--completion-fish"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _ := captureOutput(t, func() {
				code := run([]string{tt.arg})
				if code != 0 {
					t.Fatalf("exit code %d for %s", code, tt.arg)
				}
			})
			for _, flag := range requiredFlags {
				flagName := strings.TrimLeft(flag, "-")
				if !strings.Contains(stdout, flagName) {
					t.Errorf("%s completion missing flag %s", tt.name, flag)
				}
			}
		})
	}
}
