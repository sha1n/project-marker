package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	matchAll = "all"
	matchAny = "any"
)

var errFileFound = errors.New("file found")

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
	match, err := resolveMatchMode(match)
	if err != nil {
		return nil, err
	}
	return &HasSubdirectoryRule{
		Subdirectories: values,
		Match:          match,
		ApplyTag:       applyTag,
	}, nil
}

func (r *HasSubdirectoryRule) Evaluate(dirPath string) (bool, string, error) {
	if r.Match == matchAny {
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

// SubdirectoryHasFilesRule checks that subdirectories contain at least one
// non-hidden regular file at any depth.
// Match mode "all" (default) requires every listed subdirectory to qualify.
// Match mode "any" requires at least one to qualify.
type SubdirectoryHasFilesRule struct {
	Subdirectories []string
	Match          string
	ApplyTag       string
}

// NewSubdirectoryHasFilesRule creates a SubdirectoryHasFilesRule.
func NewSubdirectoryHasFilesRule(values []string, match string, applyTag string) (TagRule, error) {
	match, err := resolveMatchMode(match)
	if err != nil {
		return nil, err
	}
	return &SubdirectoryHasFilesRule{
		Subdirectories: values,
		Match:          match,
		ApplyTag:       applyTag,
	}, nil
}

func (r *SubdirectoryHasFilesRule) Evaluate(dirPath string) (bool, string, error) {
	requireAll := r.Match != matchAny
	for _, sub := range r.Subdirectories {
		qualifies, err := subdirectoryHasFiles(dirPath, sub)
		if err != nil {
			return false, "", err
		}
		if qualifies && !requireAll {
			return true, r.ApplyTag, nil
		}
		if !qualifies && requireAll {
			return false, "", nil
		}
	}
	if requireAll {
		return true, r.ApplyTag, nil
	}
	return false, "", nil
}

func subdirectoryHasFiles(parent, name string) (bool, error) {
	if !isSubdirectory(parent, name) {
		return false, nil
	}
	root := filepath.Join(parent, name)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		hidden := strings.HasPrefix(d.Name(), ".")
		if d.IsDir() {
			if hidden && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !hidden && d.Type().IsRegular() {
			return errFileFound
		}
		return nil
	})
	if errors.Is(err, errFileFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("scanning %q for files: %w", root, err)
	}
	return false, nil
}

func resolveMatchMode(match string) (string, error) {
	if match == "" {
		return matchAll, nil
	}
	if match != matchAll && match != matchAny {
		return "", fmt.Errorf("invalid match mode %q: must be \"all\" or \"any\"", match)
	}
	return match, nil
}

func isSubdirectory(parent, name string) bool {
	info, err := os.Stat(filepath.Join(parent, name))
	if err != nil {
		return false
	}
	return info.IsDir()
}
