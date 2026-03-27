package scanner

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"

	"github.com/sha1n/project-marker/internal/config"
)

// Tagger applies or removes tags on filesystem paths.
type Tagger interface {
	Apply(path, tag string) error
	Remove(path, tag string) error
}

// Scanner walks directories and evaluates targets against configured indicators and rules.
type Scanner struct {
	Targets    []config.ResolvedTarget
	Tagger     Tagger
	RemoveMode bool
}

// Result tracks what the scanner did for a single directory.
type Result struct {
	Path       string
	TargetName string
	Tag        string
	Action     string // "tagged", "untagged", "skipped", "already_tagged"
}

// Scan walks the given root directories, evaluating indicators and rules.
func (s *Scanner) Scan(roots []string) ([]Result, error) {
	var results []Result

	for _, root := range roots {
		r, err := s.scanRoot(root)
		if err != nil {
			return results, fmt.Errorf("scanning %s: %w", root, err)
		}
		results = append(results, r...)
	}

	return results, nil
}

func (s *Scanner) scanRoot(root string) ([]Result, error) {
	var results []Result

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("warning: %s: %v", path, err)
			return nil // Continue scanning
		}

		if !d.IsDir() {
			return nil
		}

		for _, target := range s.Targets {
			matched, matchErr := evaluateIndicators(path, target)
			if matchErr != nil {
				log.Printf("warning: indicator error at %s: %v", path, matchErr)
				continue
			}
			if !matched {
				continue
			}

			// This directory matches a target — evaluate rules
			ruleResults := s.evaluateRules(path, target)
			results = append(results, ruleResults...)

			// Skip descending into this project directory
			return filepath.SkipDir
		}

		return nil
	})

	return results, err
}

func evaluateIndicators(dirPath string, target config.ResolvedTarget) (bool, error) {
	for _, ind := range target.Indicators {
		match, err := ind.IsMatch(dirPath)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

func (s *Scanner) evaluateRules(dirPath string, target config.ResolvedTarget) []Result {
	var results []Result

	for _, rule := range target.Rules {
		matched, tag, err := rule.Evaluate(dirPath)
		if err != nil {
			log.Printf("warning: rule error at %s: %v", dirPath, err)
			continue
		}
		if !matched {
			continue
		}

		result := Result{
			Path:       dirPath,
			TargetName: target.Name,
			Tag:        tag,
		}

		if s.RemoveMode {
			if err := s.Tagger.Remove(dirPath, tag); err != nil {
				log.Printf("warning: failed to remove tag %q from %s: %v", tag, dirPath, err)
				result.Action = "skipped"
			} else {
				result.Action = "untagged"
			}
		} else {
			if err := s.Tagger.Apply(dirPath, tag); err != nil {
				log.Printf("warning: failed to apply tag %q to %s: %v", tag, dirPath, err)
				result.Action = "skipped"
			} else {
				result.Action = "tagged"
			}
		}

		results = append(results, result)
	}

	return results
}
