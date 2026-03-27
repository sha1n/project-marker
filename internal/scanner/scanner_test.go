package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sha1n/project-marker/internal/config"
	"github.com/sha1n/project-marker/internal/engine"
)

// mockTagger records Apply/Remove calls for assertions.
type mockTagger struct {
	applied []tagCall
	removed []tagCall
}

type tagCall struct {
	path string
	tag  string
}

func (m *mockTagger) Apply(path, tag string) error {
	m.applied = append(m.applied, tagCall{path, tag})
	return nil
}

func (m *mockTagger) Remove(path, tag string) error {
	m.removed = append(m.removed, tagCall{path, tag})
	return nil
}

func setupCubaseTarget(t *testing.T) config.ResolvedTarget {
	t.Helper()
	registry := engine.NewRegistry()

	ind, err := registry.CreateIndicator("file_extension", ".cpr")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := registry.CreateRule("has_subdirectory", []string{"Mixdown"}, "any", "Blue")
	if err != nil {
		t.Fatal(err)
	}

	return config.ResolvedTarget{
		Name:       "Cubase",
		Indicators: []engine.Indicator{ind},
		Rules:      []engine.TagRule{rule},
	}
}

func TestScan_FindsNestedProject(t *testing.T) {
	root := t.TempDir()

	// Create nested Cubase project: root/Music/Track1/Track1.cpr + root/Music/Track1/Mixdown/
	projectDir := filepath.Join(root, "Music", "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != projectDir {
		t.Errorf("expected path %s, got %s", projectDir, results[0].Path)
	}
	if results[0].Tag != "Blue" {
		t.Errorf("expected tag Blue, got %s", results[0].Tag)
	}
	if results[0].Action != "tagged" {
		t.Errorf("expected action tagged, got %s", results[0].Action)
	}
	if len(tagger.applied) != 1 {
		t.Errorf("expected 1 apply call, got %d", len(tagger.applied))
	}
}

func TestScan_NoMatchWithoutMixdown(t *testing.T) {
	root := t.TempDir()

	// Cubase project without Mixdown
	projectDir := filepath.Join(root, "Track2")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track2.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for project without Mixdown, got %d", len(results))
	}
}

func TestScan_RemoveMode(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "untagged" {
		t.Errorf("expected action untagged, got %s", results[0].Action)
	}
	if len(tagger.removed) != 1 {
		t.Errorf("expected 1 remove call, got %d", len(tagger.removed))
	}
}

func TestScan_SkipDirOptimization(t *testing.T) {
	root := t.TempDir()

	// Create a project with a nested sub-project that should NOT be found
	outerProject := filepath.Join(root, "Outer")
	innerProject := filepath.Join(outerProject, "Inner")
	if err := os.MkdirAll(filepath.Join(innerProject, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outerProject, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outerProject, "Outer.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(innerProject, "Inner.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	// Only the outer project should be found — inner is skipped via SkipDir
	if len(results) != 1 {
		t.Fatalf("expected 1 result (SkipDir optimization), got %d", len(results))
	}
	if results[0].Path != outerProject {
		t.Errorf("expected outer project, got %s", results[0].Path)
	}
}

func TestScan_UnreadableDirectory(t *testing.T) {
	root := t.TempDir()

	// Create a valid project
	projectDir := filepath.Join(root, "Track")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Create an unreadable directory as a sibling
	restricted := filepath.Join(root, "aaa-restricted")
	if err := os.Mkdir(restricted, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(restricted, 0755) }()

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	// Should still find the valid project despite the unreadable dir
	if len(results) != 1 {
		t.Fatalf("expected 1 result despite unreadable dir, got %d", len(results))
	}
}

func TestScan_MultipleRoots(t *testing.T) {
	root1 := t.TempDir()
	root2 := t.TempDir()

	for _, root := range []string{root1, root2} {
		projectDir := filepath.Join(root, "Track")
		if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "Track.cpr"), []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root1, root2})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results from 2 roots, got %d", len(results))
	}
}
