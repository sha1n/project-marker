package engine

import (
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
