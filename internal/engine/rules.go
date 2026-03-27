package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

// HasSubdirectoryRule checks for the presence of subdirectories.
// Match mode "all" (default) requires all listed subdirectories to exist.
// Match mode "any" requires at least one to exist.
type HasSubdirectoryRule struct {
	Subdirectories []string
	Match          string
	ApplyTag       string
}

// NewHasSubdirectoryRule creates a HasSubdirectoryRule.
func NewHasSubdirectoryRule(values []string, match string, applyTag string) (TagRule, error) {
	if match == "" {
		match = "all"
	}
	if match != "all" && match != "any" {
		return nil, fmt.Errorf("invalid match mode %q: must be \"all\" or \"any\"", match)
	}
	return &HasSubdirectoryRule{
		Subdirectories: values,
		Match:          match,
		ApplyTag:       applyTag,
	}, nil
}

func (r *HasSubdirectoryRule) Evaluate(dirPath string) (bool, string, error) {
	if r.Match == "any" {
		return r.evaluateAny(dirPath)
	}
	return r.evaluateAll(dirPath)
}

func (r *HasSubdirectoryRule) evaluateAll(dirPath string) (bool, string, error) {
	for _, sub := range r.Subdirectories {
		if !isSubdirectory(dirPath, sub) {
			return false, "", nil
		}
	}
	return true, r.ApplyTag, nil
}

func (r *HasSubdirectoryRule) evaluateAny(dirPath string) (bool, string, error) {
	for _, sub := range r.Subdirectories {
		if isSubdirectory(dirPath, sub) {
			return true, r.ApplyTag, nil
		}
	}
	return false, "", nil
}

func isSubdirectory(parent, name string) bool {
	info, err := os.Stat(filepath.Join(parent, name))
	if err != nil {
		return false
	}
	return info.IsDir()
}
