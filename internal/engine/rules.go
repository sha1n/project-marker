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
	return evaluateSubdirectories(dirPath, r.Subdirectories, r.Match != matchAny, r.ApplyTag, isSubdirectory)
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
	return evaluateSubdirectories(dirPath, r.Subdirectories, r.Match != matchAny, r.ApplyTag, subdirectoryHasFiles)
}

func evaluateSubdirectories(dirPath string, subdirs []string, requireAll bool, applyTag string, qualifies func(parent, name string) (bool, error)) (bool, string, error) {
	var unknown []error
	for _, sub := range subdirs {
		ok, err := qualifies(dirPath, sub)
		switch {
		case err != nil:
			// An undecided subdirectory must not abort evaluation: a later one
			// may still settle the outcome regardless of it.
			unknown = append(unknown, err)
		case ok && !requireAll:
			return true, applyTag, nil
		case !ok && requireAll:
			return false, "", nil
		}
	}
	if len(unknown) > 0 {
		return false, "", errors.Join(unknown...)
	}
	if requireAll {
		return true, applyTag, nil
	}
	return false, "", nil
}

func subdirectoryHasFiles(parent, name string) (bool, error) {
	root := filepath.Join(parent, name)
	// Lstat rather than isSubdirectory: a symlinked subdirectory must not
	// qualify, consistent with symlinks inside the tree being ignored.
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking %q: %w", root, err)
	}
	if !info.IsDir() {
		return false, nil
	}
	var readErrs []error
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Record instead of aborting: a visible file elsewhere in the tree
			// still qualifies the subdirectory. Returning nil skips the
			// contents of the directory that failed to read.
			readErrs = append(readErrs, err)
			return nil
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
	if len(readErrs) > 0 {
		return false, fmt.Errorf("scanning %q for files: %w", root, errors.Join(readErrs...))
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

func isSubdirectory(parent, name string) (bool, error) {
	path := filepath.Join(parent, name)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking %q: %w", path, err)
	}
	return info.IsDir(), nil
}
