package config

import (
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
	if len(targets[0].Rules) != 1 {
		t.Errorf("expected 1 rule for Cubase, got %d", len(targets[0].Rules))
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
