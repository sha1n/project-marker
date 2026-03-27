package engine

import (
	"testing"
)

func TestRegistryExhaustiveness(t *testing.T) {
	r := NewRegistry()

	for _, name := range AllSupportedIndicators() {
		ind, err := r.CreateIndicator(name, "test-value")
		if err != nil {
			t.Errorf("indicator %q failed to create: %v", name, err)
		}
		if ind == nil {
			t.Errorf("indicator %q returned nil", name)
		}
	}

	for _, name := range AllSupportedRules() {
		rule, err := r.CreateRule(name, []string{"test"}, "all", "Blue")
		if err != nil {
			t.Errorf("rule %q failed to create: %v", name, err)
		}
		if rule == nil {
			t.Errorf("rule %q returned nil", name)
		}
	}
}

func TestCreateIndicator_UnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.CreateIndicator("nonexistent", "value")
	if err == nil {
		t.Error("expected error for unknown indicator type")
	}
}

func TestCreateRule_UnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.CreateRule("nonexistent", []string{"v"}, "all", "Blue")
	if err == nil {
		t.Error("expected error for unknown rule type")
	}
}
