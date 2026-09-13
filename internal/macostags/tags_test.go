//go:build darwin

package macostags

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/unix"
	"howett.net/plist"
)

func TestSetAndGetTags(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tags := []string{"Blue", "Red"}
	if err := SetTags(file, tags); err != nil {
		t.Fatalf("SetTags failed: %v", err)
	}

	got, err := GetTags(file)
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}

	if len(got) != 2 || got[0] != "Blue" || got[1] != "Red" {
		t.Errorf("expected [Blue Red], got %v", got)
	}
}

func TestGetTags_NoTags(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "clean.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tags, err := GetTags(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tags != nil {
		t.Errorf("expected nil tags, got %v", tags)
	}
}

func TestAddTag(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := AddTag(file, "Blue"); err != nil {
		t.Fatalf("AddTag failed: %v", err)
	}

	// Add same tag again (idempotent)
	if err := AddTag(file, "Blue"); err != nil {
		t.Fatalf("AddTag (duplicate) failed: %v", err)
	}

	tags, err := GetTags(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "Blue" {
		t.Errorf("expected [Blue], got %v", tags)
	}

	// Add a different tag
	if err := AddTag(file, "Red"); err != nil {
		t.Fatalf("AddTag (Red) failed: %v", err)
	}

	tags, err = GetTags(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", tags)
	}
}

func TestRemoveTag(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SetTags(file, []string{"Blue", "Red"}); err != nil {
		t.Fatal(err)
	}

	if err := RemoveTag(file, "Blue"); err != nil {
		t.Fatalf("RemoveTag failed: %v", err)
	}

	tags, err := GetTags(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "Red" {
		t.Errorf("expected [Red], got %v", tags)
	}
}

func TestRemoveTag_LastTag(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SetTags(file, []string{"Blue"}); err != nil {
		t.Fatal(err)
	}

	if err := RemoveTag(file, "Blue"); err != nil {
		t.Fatalf("RemoveTag failed: %v", err)
	}

	tags, err := GetTags(file)
	if err != nil {
		t.Fatal(err)
	}
	if tags != nil {
		t.Errorf("expected nil tags after removing last tag, got %v", tags)
	}
}

func TestRemoveTag_NotPresent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveTag(file, "Blue"); err != nil {
		t.Fatalf("RemoveTag on untagged file failed: %v", err)
	}
}

func TestGetTags_MissingFile(t *testing.T) {
	tags, err := GetTags("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if tags != nil {
		t.Errorf("expected nil tags for missing file, got %v", tags)
	}
}

func TestTagger_HasTag(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &Tagger{}

	// No tags yet
	has, err := tagger.HasTag(file, "Blue")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if has {
		t.Error("expected HasTag=false for untagged file")
	}

	// Add a tag and check
	if err := tagger.Apply(file, "Blue"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	has, err = tagger.HasTag(file, "Blue")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if !has {
		t.Error("expected HasTag=true after Apply")
	}

	// Check for a different tag
	has, err = tagger.HasTag(file, "Red")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if has {
		t.Error("expected HasTag=false for non-applied tag")
	}
}

func TestTagger_ApplyAndRemove(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &Tagger{}

	if err := tagger.Apply(file, "Blue"); err != nil {
		t.Fatalf("Tagger.Apply failed: %v", err)
	}

	tags, err := GetTags(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "Blue" {
		t.Errorf("expected [Blue], got %v", tags)
	}

	if err := tagger.Remove(file, "Blue"); err != nil {
		t.Fatalf("Tagger.Remove failed: %v", err)
	}

	tags, err = GetTags(file)
	if err != nil {
		t.Fatal(err)
	}
	if tags != nil {
		t.Errorf("expected nil tags after remove, got %v", tags)
	}
}

const finderInfoKey = "com.apple.FinderInfo"

func rawTagEntries(t *testing.T, path string) []string {
	t.Helper()
	size, err := unix.Getxattr(path, xattrKey, nil)
	if errors.Is(err, unix.ENOATTR) {
		return nil
	}
	if err != nil {
		t.Fatalf("getxattr %s size: %v", xattrKey, err)
	}
	buf := make([]byte, size)
	if _, err := unix.Getxattr(path, xattrKey, buf); err != nil {
		t.Fatalf("getxattr %s: %v", xattrKey, err)
	}
	var entries []string
	if _, err := plist.Unmarshal(buf, &entries); err != nil {
		t.Fatalf("unmarshal %s: %v", xattrKey, err)
	}
	return entries
}

func writeRawTagEntries(t *testing.T, path string, entries []string) {
	t.Helper()
	data, err := plist.Marshal(entries, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Setxattr(path, xattrKey, data, 0); err != nil {
		t.Fatalf("setxattr %s: %v", xattrKey, err)
	}
}

// labelColor returns the Finder label color index stored in FinderInfo
// (byte 9 of the 32-byte record holds colorIndex << 1), or 0 when absent.
func labelColor(t *testing.T, path string) int {
	t.Helper()
	buf := make([]byte, 32)
	n, err := unix.Getxattr(path, finderInfoKey, buf)
	if errors.Is(err, unix.ENOATTR) {
		return 0
	}
	if err != nil {
		t.Fatalf("getxattr %s: %v", finderInfoKey, err)
	}
	if n < 10 {
		t.Fatalf("%s too short: %d bytes", finderInfoKey, n)
	}
	return int(buf[9]>>1) & 7
}

func newTestDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func assertRawTagEntries(t *testing.T, path string, want []string) {
	t.Helper()
	if got := rawTagEntries(t, path); !slices.Equal(got, want) {
		t.Errorf("raw tag entries: expected %q, got %q", want, got)
	}
}

func assertLabelColor(t *testing.T, path string, want int) {
	t.Helper()
	if got := labelColor(t, path); got != want {
		t.Errorf("label color: expected %d, got %d", want, got)
	}
}

func assertTags(t *testing.T, path string, want []string) {
	t.Helper()
	got, err := GetTags(path)
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}
	if !slices.Equal(got, want) || (want == nil && got != nil) {
		t.Errorf("tags: expected %q, got %q", want, got)
	}
}

func TestSetTags_Directory_StoresColorMetadataAndLabel(t *testing.T) {
	dir := newTestDir(t, "project")

	if err := SetTags(dir, []string{"Blue", "Green"}); err != nil {
		t.Fatalf("SetTags failed: %v", err)
	}

	assertRawTagEntries(t, dir, []string{"Blue\n4", "Green\n2"})
	assertLabelColor(t, dir, 2)
	assertTags(t, dir, []string{"Blue", "Green"})
}

func TestTagger_Apply_Directory_StoresColorMetadataAndLabel(t *testing.T) {
	dir := newTestDir(t, "project")
	tagger := &Tagger{}

	if err := tagger.Apply(dir, "Blue"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	assertRawTagEntries(t, dir, []string{"Blue\n4"})
	assertLabelColor(t, dir, 4)
}

func TestTagger_Apply_RepairsLegacyBareNameTags(t *testing.T) {
	dir := newTestDir(t, "project")
	tagger := &Tagger{}

	// Older projmark versions wrote bare names, which Finder does not color.
	writeRawTagEntries(t, dir, []string{"Blue"})

	has, err := tagger.HasTag(dir, "Blue")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if has {
		t.Error("expected HasTag=false for legacy bare-name tag so the scanner re-applies it")
	}

	if err := tagger.Apply(dir, "Blue"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	assertRawTagEntries(t, dir, []string{"Blue\n4"})
	assertLabelColor(t, dir, 4)
	assertTags(t, dir, []string{"Blue"})

	has, err = tagger.HasTag(dir, "Blue")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if !has {
		t.Error("expected HasTag=true after repair")
	}
}

func TestTagger_Apply_RepairsDuplicateLegacyEntries(t *testing.T) {
	dir := newTestDir(t, "project")
	tagger := &Tagger{}

	// Older projmark versions appended a bare name next to Finder's colored entry.
	writeRawTagEntries(t, dir, []string{"Orange\n7", "Blue", "Orange"})

	if err := tagger.Apply(dir, "Orange"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	assertRawTagEntries(t, dir, []string{"Orange\n7", "Blue\n4"})
	assertLabelColor(t, dir, 4)
	assertTags(t, dir, []string{"Orange", "Blue"})
}

func TestTagger_Apply_RepairsDuplicateEntriesWithoutLegacy(t *testing.T) {
	dir := newTestDir(t, "project")
	tagger := &Tagger{}

	writeRawTagEntries(t, dir, []string{"Orange\n7", "Orange\n7"})

	has, err := tagger.HasTag(dir, "Orange")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if has {
		t.Error("expected HasTag=false for duplicate colored entries")
	}

	if err := tagger.Apply(dir, "Orange"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	assertRawTagEntries(t, dir, []string{"Orange\n7"})
	assertLabelColor(t, dir, 7)

	has, err = tagger.HasTag(dir, "Orange")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if !has {
		t.Error("expected HasTag=true after repair")
	}

	assertTags(t, dir, []string{"Orange"})
}

func TestFinderWrittenTags(t *testing.T) {
	dir := newTestDir(t, "project")
	tagger := &Tagger{}
	writeRawTagEntries(t, dir, []string{"Orange\n7"})

	assertTags(t, dir, []string{"Orange"})

	has, err := tagger.HasTag(dir, "Orange")
	if err != nil {
		t.Fatalf("HasTag failed: %v", err)
	}
	if !has {
		t.Error("expected HasTag=true for Finder-written tag")
	}

	if err := AddTag(dir, "Orange"); err != nil {
		t.Fatalf("AddTag failed: %v", err)
	}
	if got := rawTagEntries(t, dir); len(got) != 1 {
		t.Errorf("expected exactly one raw entry after AddTag, got %q", got)
	}
	assertTags(t, dir, []string{"Orange"})

	if err := RemoveTag(dir, "Orange"); err != nil {
		t.Fatalf("RemoveTag failed: %v", err)
	}
	assertTags(t, dir, nil)
	assertLabelColor(t, dir, 0)
}

func TestTagger_Remove_LastTag_ClearsLabelColor(t *testing.T) {
	dir := newTestDir(t, "project")
	tagger := &Tagger{}

	if err := tagger.Apply(dir, "Blue"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	assertLabelColor(t, dir, 4)

	if err := tagger.Remove(dir, "Blue"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	assertTags(t, dir, nil)
	assertLabelColor(t, dir, 0)
}

func TestSetTags_CustomTagName(t *testing.T) {
	dir := newTestDir(t, "project")

	if err := SetTags(dir, []string{"ProjmarkCustomTag"}); err != nil {
		t.Fatalf("SetTags failed: %v", err)
	}
	assertTags(t, dir, []string{"ProjmarkCustomTag"})
}

func TestTagger_Apply_PathWithSpacesAndNonASCII(t *testing.T) {
	dir := newTestDir(t, "שיחה בשניים (דואט)")
	tagger := &Tagger{}

	if err := tagger.Apply(dir, "Green"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	assertTags(t, dir, []string{"Green"})
	assertLabelColor(t, dir, 2)
}

func TestSetTags_MissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := SetTags(missing, []string{"Blue"}); err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestSetTags_MissingPath_ReportsNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	err := SetTags(missing, []string{"Blue"})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got: %v", err)
	}
	wantMsg := fmt.Sprintf("cannot update Finder tags on %q: no such file or directory", missing)
	if err.Error() != wantMsg {
		t.Errorf("error message: expected %q, got %q", wantMsg, err.Error())
	}
}

func TestTaggerApply_PermissionDenied_ReportsFriendlyError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}
	dir := newTestDir(t, "project")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	})

	tagger := &Tagger{}
	err := tagger.Apply(dir, "Blue")
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected fs.ErrPermission, got: %v", err)
	}
	wantMsg := fmt.Sprintf("cannot update Finder tags on %q: permission denied", dir)
	if err.Error() != wantMsg {
		t.Errorf("error message: expected %q, got %q", wantMsg, err.Error())
	}
}

func TestGetTags_ParentPermissionDenied_ReportsFriendlyError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(parent, 0o755); err != nil {
			t.Fatal(err)
		}
	})

	_, err := GetTags(child)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected fs.ErrPermission, got: %v", err)
	}
	wantMsg := fmt.Sprintf("cannot read Finder tags on %q: permission denied", child)
	if err.Error() != wantMsg {
		t.Errorf("error message: expected %q, got %q", wantMsg, err.Error())
	}
}

func TestGetTags_MalformedTagsXattr_ReportsFriendlyError(t *testing.T) {
	dir := newTestDir(t, "project")
	if err := unix.Setxattr(dir, xattrKey, []byte("not a plist"), 0); err != nil {
		t.Fatal(err)
	}

	_, err := GetTags(dir)
	wantMsg := fmt.Sprintf("cannot read Finder tags on %q: the stored tags are malformed", dir)
	if err == nil || err.Error() != wantMsg {
		t.Errorf("error message: expected %q, got %v", wantMsg, err)
	}
}
