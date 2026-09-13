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

func TestAllSupportedRules_IncludesSubdirectoryHasFiles(t *testing.T) {
	// The constant value is referenced verbatim by YAML configs.
	if RuleSubdirectoryHasFiles != "subdirectory_has_files" {
		t.Errorf("expected RuleSubdirectoryHasFiles to be %q, got %q", "subdirectory_has_files", RuleSubdirectoryHasFiles)
	}

	found := false
	for _, name := range AllSupportedRules() {
		if name == RuleSubdirectoryHasFiles {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected AllSupportedRules to include %q", RuleSubdirectoryHasFiles)
	}
}

func TestCreateRule_SubdirectoryHasFiles(t *testing.T) {
	r := NewRegistry()
	rule, err := r.CreateRule(RuleSubdirectoryHasFiles, []string{"Audio"}, "any", "Green")
	if err != nil {
		t.Fatalf("failed to create %q rule: %v", RuleSubdirectoryHasFiles, err)
	}
	shfRule, ok := rule.(*SubdirectoryHasFilesRule)
	if !ok {
		t.Fatalf("expected *SubdirectoryHasFilesRule, got %T", rule)
	}
	if shfRule.Match != "any" {
		t.Errorf("expected match mode 'any', got %q", shfRule.Match)
	}
	if shfRule.ApplyTag != "Green" {
		t.Errorf("expected apply tag Green, got %q", shfRule.ApplyTag)
	}
}
