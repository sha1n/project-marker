package engine

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestHasSubdirectoryRule_AllMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Exported Files"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{"Mixdown", "Exported Files"},
		Match:          "all",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match when all subdirectories exist")
	}
	if tag != "Blue" {
		t.Errorf("expected tag Blue, got %q", tag)
	}
}

func TestHasSubdirectoryRule_AllPartialMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{"Mixdown", "Exported Files"},
		Match:          "all",
		ApplyTag:       "Blue",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when only a subset of subdirectories exist")
	}
}

func TestHasSubdirectoryRule_AnyMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{"Mixdown", "Exported Files"},
		Match:          "any",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match when at least one subdirectory exists")
	}
	if tag != "Blue" {
		t.Errorf("expected tag Blue, got %q", tag)
	}
}

func TestHasSubdirectoryRule_AnyNoneMatch(t *testing.T) {
	dir := t.TempDir()

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{"Mixdown", "Exported Files"},
		Match:          "any",
		ApplyTag:       "Blue",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when no subdirectories exist")
	}
}

func TestHasSubdirectoryRule_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Mixdown"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{"Mixdown"},
		Match:          "all",
		ApplyTag:       "Blue",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when Mixdown is a file, not a directory")
	}
}

func TestHasSubdirectoryRule_DefaultMatchMode(t *testing.T) {
	rule, err := NewHasSubdirectoryRule([]string{"a"}, "", "Blue")
	if err != nil {
		t.Fatal(err)
	}
	hsRule := rule.(*HasSubdirectoryRule)
	if hsRule.Match != "all" {
		t.Errorf("expected default match mode 'all', got %q", hsRule.Match)
	}
}

func TestHasSubdirectoryRule_InvalidMatchMode(t *testing.T) {
	_, err := NewHasSubdirectoryRule([]string{"a"}, "invalid", "Blue")
	if err == nil {
		t.Error("expected error for invalid match mode")
	}
}

// setupHasSubdirectoryPermissionFixture builds a project dir containing a
// real "Mixdown" subdirectory and a "Locked/Inner" subdirectory whose parent
// is chmod 0000, so stat-ing "Locked/Inner" fails with a permission error
// rather than a not-exist error. "Missing" is left absent.
func setupHasSubdirectoryPermissionFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "Locked")
	if err := os.MkdirAll(filepath.Join(locked, "Inner"), 0755); err != nil {
		t.Fatal(err)
	}
	lockTestDir(t, locked)
	return dir
}

func TestHasSubdirectoryRule_AllErrorsOnUnreadableSubdirectory(t *testing.T) {
	dir := setupHasSubdirectoryPermissionFixture(t)

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{filepath.Join("Locked", "Inner")},
		Match:          "all",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err == nil {
		t.Fatal("expected error when a subdirectory cannot be statted")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("expected error to wrap fs.ErrPermission, got %v", err)
	}
	if matched {
		t.Error("expected no match when a subdirectory cannot be statted")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestHasSubdirectoryRule_AllMissingSubdirectoryShortCircuitsOverError(t *testing.T) {
	dir := setupHasSubdirectoryPermissionFixture(t)

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{filepath.Join("Locked", "Inner"), "Missing"},
		Match:          "all",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error when a subdirectory definitively does not qualify, got %v", err)
	}
	if matched {
		t.Error("expected no match when a listed subdirectory does not exist")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestHasSubdirectoryRule_AllErrorsWhenNoDefinitiveMismatch(t *testing.T) {
	dir := setupHasSubdirectoryPermissionFixture(t)

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{filepath.Join("Locked", "Inner"), "Mixdown"},
		Match:          "all",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err == nil {
		t.Fatal("expected error when the only other subdirectory qualifies but one could not be checked")
	}
	if matched {
		t.Error("expected no match when a subdirectory could not be checked")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestHasSubdirectoryRule_AnyQualifyingSubdirectoryOverridesError(t *testing.T) {
	dir := setupHasSubdirectoryPermissionFixture(t)

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{filepath.Join("Locked", "Inner"), "Mixdown"},
		Match:          "any",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error when another subdirectory qualifies in any mode, got %v", err)
	}
	if !matched {
		t.Error("expected match when another subdirectory qualifies")
	}
	if tag != "Blue" {
		t.Errorf("expected tag Blue, got %q", tag)
	}
}

func TestHasSubdirectoryRule_AnyErrorsWhenNoSubdirectoryQualifies(t *testing.T) {
	dir := setupHasSubdirectoryPermissionFixture(t)

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{filepath.Join("Locked", "Inner"), "Missing"},
		Match:          "any",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err == nil {
		t.Fatal("expected error when no subdirectory qualifies and one could not be checked")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("expected error to wrap fs.ErrPermission, got %v", err)
	}
	if matched {
		t.Error("expected no match when no subdirectory qualifies")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestHasSubdirectoryRule_SymlinkedSubdirectoryFollowed(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, "Mixdown")); err != nil {
		t.Fatal(err)
	}

	rule := &HasSubdirectoryRule{
		Subdirectories: []string{"Mixdown"},
		Match:          "all",
		ApplyTag:       "Blue",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error for a symlinked subdirectory pointing to a real directory, got %v", err)
	}
	if !matched {
		t.Error("expected match when Mixdown is a symlink to a real directory")
	}
	if tag != "Blue" {
		t.Errorf("expected tag Blue, got %q", tag)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSubdirectoryHasFilesRule_TopLevelFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio", "take.wav"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match when Audio contains a visible file")
	}
	if tag != "Green" {
		t.Errorf("expected tag Green, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_NestedFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio", "Edits", "Takes", "take.wav"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match when Audio contains a file in a nested subfolder")
	}
	if tag != "Green" {
		t.Errorf("expected tag Green, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_OnlyHiddenFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio", ".DS_Store"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when Audio contains only hidden files")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_FileOnlyInHiddenSubdirectory(t *testing.T) {
	dir := t.TempDir()
	// The file itself is visible; only its parent directory is hidden.
	writeTestFile(t, filepath.Join(dir, "Audio", ".cache", "x.wav"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when files exist only inside a hidden subdirectory")
	}
}

func TestSubdirectoryHasFilesRule_EmptySubdirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Audio", "Empty"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when Audio contains no files")
	}
}

func TestSubdirectoryHasFilesRule_MissingSubdirectory(t *testing.T) {
	dir := t.TempDir()

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error for missing subdirectory, got %v", err)
	}
	if matched {
		t.Error("expected no match when Audio does not exist")
	}
}

func TestSubdirectoryHasFilesRule_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error when Audio is a file, got %v", err)
	}
	if matched {
		t.Error("expected no match when Audio is a file, not a directory")
	}
}

func TestSubdirectoryHasFilesRule_AllPartialMismatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio", "take.wav"))
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio", "Mixdown"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when only a subset of subdirectories contain files")
	}
}

func TestSubdirectoryHasFilesRule_AllMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio", "take.wav"))
	writeTestFile(t, filepath.Join(dir, "Mixdown", "mix.wav"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio", "Mixdown"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match when all subdirectories contain files")
	}
	if tag != "Green" {
		t.Errorf("expected tag Green, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_AnyMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio", "take.wav"))
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Mixdown", "Audio"},
		Match:          "any",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match when at least one subdirectory contains files")
	}
	if tag != "Green" {
		t.Errorf("expected tag Green, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_AnyNoneMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Audio", ".DS_Store"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Mixdown", "Audio"},
		Match:          "any",
		ApplyTag:       "Green",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when no subdirectory contains files")
	}
}

func TestSubdirectoryHasFilesRule_DefaultMatchMode(t *testing.T) {
	rule, err := NewSubdirectoryHasFilesRule([]string{"a"}, "", "Green")
	if err != nil {
		t.Fatal(err)
	}
	shfRule, ok := rule.(*SubdirectoryHasFilesRule)
	if !ok {
		t.Fatalf("expected *SubdirectoryHasFilesRule, got %T", rule)
	}
	if shfRule.Match != "all" {
		t.Errorf("expected default match mode 'all', got %q", shfRule.Match)
	}
	if shfRule.ApplyTag != "Green" {
		t.Errorf("expected apply tag Green, got %q", shfRule.ApplyTag)
	}
	if len(shfRule.Subdirectories) != 1 || shfRule.Subdirectories[0] != "a" {
		t.Errorf("expected subdirectories [a], got %v", shfRule.Subdirectories)
	}
}

func TestSubdirectoryHasFilesRule_InvalidMatchMode(t *testing.T) {
	_, err := NewSubdirectoryHasFilesRule([]string{"a"}, "invalid", "Green")
	if err == nil {
		t.Error("expected error for invalid match mode")
	}
}

func TestSubdirectoryHasFilesRule_SymlinkNotCounted(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.wav")
	writeTestFile(t, target)
	if err := os.Mkdir(filepath.Join(dir, "Audio"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "Audio", "link.wav")); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, _, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when Audio contains only a symlink to a file")
	}
}

func TestSubdirectoryHasFilesRule_UnreadableNestedDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks are bypassed when running as root")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "Audio", "Locked")
	writeTestFile(t, filepath.Join(locked, "take.wav"))
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatal(err)
	}
	// Restore permissions so t.TempDir cleanup can remove the tree.
	t.Cleanup(func() { _ = os.Chmod(locked, 0755) })

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	_, _, err := rule.Evaluate(dir)
	if err == nil {
		t.Error("expected error when a nested directory cannot be read")
	}
}

func lockTestDir(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission checks are bypassed when running as root")
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	// Restore permissions so t.TempDir cleanup can remove the tree.
	t.Cleanup(func() { _ = os.Chmod(path, 0755) })
}

func TestSubdirectoryHasFilesRule_UnreadableNestedDirectoryDoesNotHideSiblingFile(t *testing.T) {
	dir := t.TempDir()
	// "Locked" sorts before "take.wav", so the walk reaches the unreadable
	// directory before the visible file.
	locked := filepath.Join(dir, "Audio", "Locked")
	writeTestFile(t, filepath.Join(locked, "hidden-by-permissions.wav"))
	writeTestFile(t, filepath.Join(dir, "Audio", "take.wav"))
	lockTestDir(t, locked)

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error when a visible file exists beside an unreadable directory, got %v", err)
	}
	if !matched {
		t.Error("expected match when Audio contains a visible file beside an unreadable directory")
	}
	if tag != "Green" {
		t.Errorf("expected tag Green, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_AnyUnreadableFirstOtherHasFile(t *testing.T) {
	dir := t.TempDir()
	lockTestDir(t, filepath.Join(dir, "Audio"))
	writeTestFile(t, filepath.Join(dir, "Mixdown", "mix.wav"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio", "Mixdown"},
		Match:          "any",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error when another subdirectory qualifies in any mode, got %v", err)
	}
	if !matched {
		t.Error("expected match when another subdirectory contains a visible file")
	}
	if tag != "Green" {
		t.Errorf("expected tag Green, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_AnyUnreadableOtherEmpty(t *testing.T) {
	dir := t.TempDir()
	lockTestDir(t, filepath.Join(dir, "Audio"))
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio", "Mixdown"},
		Match:          "any",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err == nil {
		t.Error("expected error when no subdirectory qualifies and one cannot be read")
	}
	if matched {
		t.Error("expected no match when no subdirectory qualifies")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_AllUnreadableFirstOtherEmpty(t *testing.T) {
	dir := t.TempDir()
	lockTestDir(t, filepath.Join(dir, "Audio"))
	if err := os.Mkdir(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio", "Mixdown"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error when another subdirectory definitively fails in all mode, got %v", err)
	}
	if matched {
		t.Error("expected no match when a subdirectory is empty")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_AllUnreadableFirstOtherHasFile(t *testing.T) {
	dir := t.TempDir()
	lockTestDir(t, filepath.Join(dir, "Audio"))
	writeTestFile(t, filepath.Join(dir, "Mixdown", "mix.wav"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio", "Mixdown"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err == nil {
		t.Error("expected error when a subdirectory cannot be read and no other fails")
	}
	if matched {
		t.Error("expected no match when a subdirectory cannot be read")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_UnreadableSubdirectoryRoot(t *testing.T) {
	dir := t.TempDir()
	lockTestDir(t, filepath.Join(dir, "Audio"))

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err == nil {
		t.Error("expected error when the subdirectory itself cannot be read")
	}
	if matched {
		t.Error("expected no match when the subdirectory cannot be read")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_SymlinkedSubdirectoryNotFollowed(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	writeTestFile(t, filepath.Join(target, "take.wav"))
	if err := os.Symlink(target, filepath.Join(dir, "Audio")); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error for a symlinked subdirectory, got %v", err)
	}
	if matched {
		t.Error("expected no match when Audio is a symlink to a directory")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}

func TestSubdirectoryHasFilesRule_DanglingSymlinkSubdirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "Audio")); err != nil {
		t.Fatal(err)
	}

	rule := &SubdirectoryHasFilesRule{
		Subdirectories: []string{"Audio"},
		Match:          "all",
		ApplyTag:       "Green",
	}

	matched, tag, err := rule.Evaluate(dir)
	if err != nil {
		t.Fatalf("expected nil error for a dangling symlink, got %v", err)
	}
	if matched {
		t.Error("expected no match when Audio is a dangling symlink")
	}
	if tag != "" {
		t.Errorf("expected empty tag on no match, got %q", tag)
	}
}
