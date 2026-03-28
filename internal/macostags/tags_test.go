//go:build darwin

package macostags

import (
	"os"
	"path/filepath"
	"testing"
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
