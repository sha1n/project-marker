package config

import (
	_ "embed"
	"fmt"

	"github.com/sha1n/project-marker/internal/engine"
	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var defaultConfig []byte

// IndicatorConfig represents an indicator entry in the YAML.
type IndicatorConfig struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

// RuleConfig represents a rule entry in the YAML.
type RuleConfig struct {
	Type     string   `yaml:"type"`
	Match    string   `yaml:"match"`
	Value    []string `yaml:"value"`
	ApplyTag string   `yaml:"apply_tag"`
}

// TargetConfig represents a target entry in the YAML.
type TargetConfig struct {
	Name       string            `yaml:"name"`
	Indicators []IndicatorConfig `yaml:"indicators"`
	Rules      []RuleConfig      `yaml:"rules"`
}

// Config represents the top-level YAML structure.
type Config struct {
	Targets []TargetConfig `yaml:"targets"`
}

// ResolvedTarget holds instantiated indicators and rules for a target.
type ResolvedTarget struct {
	Name       string
	Indicators []engine.Indicator
	Rules      []engine.TagRule
}

// Load parses the embedded default YAML and resolves all handlers via the registry.
func Load(registry *engine.Registry) ([]ResolvedTarget, error) {
	return LoadFromBytes(defaultConfig, registry)
}

// LoadFromBytes parses arbitrary YAML bytes and resolves handlers.
func LoadFromBytes(data []byte, registry *engine.Registry) ([]ResolvedTarget, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	var targets []ResolvedTarget
	for _, tc := range cfg.Targets {
		rt, err := resolveTarget(tc, registry)
		if err != nil {
			return nil, fmt.Errorf("target %q: %w", tc.Name, err)
		}
		targets = append(targets, rt)
	}
	return targets, nil
}

func resolveTarget(tc TargetConfig, registry *engine.Registry) (ResolvedTarget, error) {
	rt := ResolvedTarget{Name: tc.Name}

	for _, ic := range tc.Indicators {
		ind, err := registry.CreateIndicator(ic.Type, ic.Value)
		if err != nil {
			return rt, fmt.Errorf("indicator: %w", err)
		}
		rt.Indicators = append(rt.Indicators, ind)
	}

	for _, rc := range tc.Rules {
		if rc.ApplyTag == "" {
			return rt, fmt.Errorf("rule of type %q has empty apply_tag", rc.Type)
		}
		rule, err := registry.CreateRule(rc.Type, rc.Value, rc.Match, rc.ApplyTag)
		if err != nil {
			return rt, fmt.Errorf("rule: %w", err)
		}
		rt.Rules = append(rt.Rules, rule)
	}

	return rt, nil
}
