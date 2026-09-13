package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sha1n/project-marker/internal/engine"
)

func TestLoadDefaultConfig(t *testing.T) {
	registry := engine.NewRegistry()
	targets, err := Load(registry)
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].Name != "Cubase" {
		t.Errorf("expected first target name 'Cubase', got %q", targets[0].Name)
	}
	if len(targets[0].Indicators) != 1 {
		t.Errorf("expected 1 indicator for Cubase, got %d", len(targets[0].Indicators))
	}
	if len(targets[0].Rules) != 2 {
		t.Errorf("expected 2 rules for Cubase, got %d", len(targets[0].Rules))
	}

	if targets[1].Name != "LUNA" {
		t.Errorf("expected second target name 'LUNA', got %q", targets[1].Name)
	}
}

func TestLoadFromBytes_MalformedYAML(t *testing.T) {
	registry := engine.NewRegistry()
	_, err := LoadFromBytes([]byte("{{{{not yaml"), registry)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestLoadFromBytes_UnknownIndicatorType(t *testing.T) {
	registry := engine.NewRegistry()
	yaml := []byte(`
targets:
  - name: "Test"
    indicators:
      - type: "nonexistent_indicator"
        value: ".foo"
    rules: []
`)
	_, err := LoadFromBytes(yaml, registry)
	if err == nil {
		t.Error("expected error for unknown indicator type")
	}
}

func TestLoadFromBytes_UnknownRuleType(t *testing.T) {
	registry := engine.NewRegistry()
	yaml := []byte(`
targets:
  - name: "Test"
    indicators:
      - type: "file_extension"
        value: ".foo"
    rules:
      - type: "nonexistent_rule"
        value: ["bar"]
        apply_tag: "Red"
`)
	_, err := LoadFromBytes(yaml, registry)
	if err == nil {
		t.Error("expected error for unknown rule type")
	}
}

func TestLoadFromBytes_ValidCustomConfig(t *testing.T) {
	registry := engine.NewRegistry()
	yaml := []byte(`
targets:
  - name: "Custom"
    indicators:
      - type: "file_exists"
        value: "marker.txt"
    rules:
      - type: "has_subdirectory"
        match: "all"
        value: ["output", "logs"]
        apply_tag: "Green"
`)
	targets, err := LoadFromBytes(yaml, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Name != "Custom" {
		t.Errorf("expected target name 'Custom', got %q", targets[0].Name)
	}
}

func TestLoadFromBytes_EmptyApplyTag(t *testing.T) {
	registry := engine.NewRegistry()
	yaml := []byte(`
targets:
  - name: Test
    indicators:
      - type: file_extension
        value: .test
    rules:
      - type: has_subdirectory
        match: any
        value: [Output]
        apply_tag: ""
`)
	_, err := LoadFromBytes(yaml, registry)
	if err == nil {
		t.Fatal("expected error for empty apply_tag")
	}
	if !strings.Contains(err.Error(), "empty apply_tag") {
		t.Errorf("expected 'empty apply_tag' in error, got: %v", err)
	}
}

func TestDefaultConfigUsesOnlyRegisteredHandlers(t *testing.T) {
	registry := engine.NewRegistry()
	_, err := Load(registry)
	if err != nil {
		t.Fatalf("default config references unregistered handler: %v", err)
	}
}

func TestLoadDefaultConfig_CubaseAudioFilesRule(t *testing.T) {
	registry := engine.NewRegistry()
	targets, err := Load(registry)
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}
	if len(targets) == 0 || targets[0].Name != "Cubase" {
		t.Fatalf("expected first target to be Cubase, got %+v", targets)
	}

	withAudio := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withAudio, "Audio"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(withAudio, "Audio", "take.wav"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	hiddenOnly := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hiddenOnly, "Audio"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenOnly, "Audio", ".DS_Store"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if tags := evaluateRuleTags(t, targets[0], withAudio); len(tags) != 1 || tags[0] != "Green" {
		t.Errorf("expected Cubase rules to yield [Green] for Audio with files, got %v", tags)
	}
	if tags := evaluateRuleTags(t, targets[0], hiddenOnly); len(tags) != 0 {
		t.Errorf("expected Cubase rules to yield no tags for Audio with only hidden files, got %v", tags)
	}
}

func evaluateRuleTags(t *testing.T, target ResolvedTarget, dir string) []string {
	t.Helper()
	var tags []string
	for _, rule := range target.Rules {
		matched, tag, err := rule.Evaluate(dir)
		if err != nil {
			t.Fatalf("rule evaluation failed for %s: %v", dir, err)
		}
		if matched {
			tags = append(tags, tag)
		}
	}
	return tags
}
