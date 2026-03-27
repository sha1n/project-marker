package engine

import "fmt"

// Indicator determines whether a directory matches a target project type.
type Indicator interface {
	IsMatch(dirPath string) (bool, error)
}

// TagRule evaluates a condition on a directory and returns the tag to apply.
type TagRule interface {
	Evaluate(dirPath string) (bool, string, error)
}

// Supported indicator type constants.
const (
	IndicatorFileExtension      = "file_extension"
	IndicatorDirectoryExtension = "directory_extension"
	IndicatorFileExists         = "file_exists"
)

// Supported rule type constants.
const (
	RuleHasSubdirectory = "has_subdirectory"
)

// IndicatorFactory creates an Indicator from a config value.
type IndicatorFactory func(value string) (Indicator, error)

// RuleFactory creates a TagRule from config values.
type RuleFactory func(values []string, match string, applyTag string) (TagRule, error)

// Registry maps type strings to factory functions.
type Registry struct {
	indicators map[string]IndicatorFactory
	rules      map[string]RuleFactory
}

// NewRegistry creates a registry pre-populated with all built-in handlers.
func NewRegistry() *Registry {
	r := &Registry{
		indicators: make(map[string]IndicatorFactory),
		rules:      make(map[string]RuleFactory),
	}

	r.RegisterIndicator(IndicatorFileExtension, NewFileExtensionIndicator)
	r.RegisterIndicator(IndicatorDirectoryExtension, NewDirectoryExtensionIndicator)
	r.RegisterIndicator(IndicatorFileExists, NewFileExistsIndicator)

	r.RegisterRule(RuleHasSubdirectory, NewHasSubdirectoryRule)

	return r
}

// RegisterIndicator adds an indicator factory to the registry.
func (r *Registry) RegisterIndicator(name string, factory IndicatorFactory) {
	r.indicators[name] = factory
}

// RegisterRule adds a rule factory to the registry.
func (r *Registry) RegisterRule(name string, factory RuleFactory) {
	r.rules[name] = factory
}

// CreateIndicator resolves and creates an indicator by type name.
func (r *Registry) CreateIndicator(typeName string, value string) (Indicator, error) {
	factory, ok := r.indicators[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown indicator type: %q", typeName)
	}
	return factory(value)
}

// CreateRule resolves and creates a rule by type name.
func (r *Registry) CreateRule(typeName string, values []string, match string, applyTag string) (TagRule, error) {
	factory, ok := r.rules[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown rule type: %q", typeName)
	}
	return factory(values, match, applyTag)
}

// AllSupportedIndicators returns all registered indicator type names.
func AllSupportedIndicators() []string {
	return []string{
		IndicatorFileExtension,
		IndicatorDirectoryExtension,
		IndicatorFileExists,
	}
}

// AllSupportedRules returns all registered rule type names.
func AllSupportedRules() []string {
	return []string{
		RuleHasSubdirectory,
	}
}
