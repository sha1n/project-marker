package scanner

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/sha1n/project-marker/internal/config"
)

// Tagger applies or removes tags on filesystem paths.
type Tagger interface {
	Apply(path, tag string) error
	Remove(path, tag string) error
}

// EventKind classifies what happened at a directory during scanning.
type EventKind int

const (
	EventEnter EventKind = iota // entering a directory (no match)
	EventMatch                  // directory matched a target + rule — tagged/untagged
	EventSkip                   // matched target but rule didn't match (no tag action)
	EventWarn                   // walk error or other warning
)

// ScanEvent describes what happened at a single directory.
type ScanEvent struct {
	Kind       EventKind
	Path       string
	TargetName string // empty for EventEnter/EventWarn
	Tag        string // empty unless EventMatch
	Action     string // "tagged"/"untagged" for EventMatch
	Message    string // for EventWarn
}

// Scanner walks directories and evaluates targets against configured indicators and rules.
type Scanner struct {
	Targets    []config.ResolvedTarget
	Tagger     Tagger
	RemoveMode bool
	Logger     *slog.Logger
	OnVisit    func(ScanEvent)
}

// Result tracks what the scanner did for a single directory.
type Result struct {
	Path       string
	TargetName string
	Tag        string
	Action     string // "tagged", "untagged", "skipped", "already_tagged"
}

func (s *Scanner) emit(e ScanEvent) {
	if s.OnVisit != nil {
		s.OnVisit(e)
	}
}

// Scan walks the given root directories, evaluating indicators and rules.
func (s *Scanner) Scan(roots []string) ([]Result, error) {
	if s.Logger == nil {
		s.Logger = slog.New(slog.DiscardHandler)
	}

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
			if d == nil {
				// Root itself is inaccessible — fatal for this root.
				return err
			}
			s.Logger.Warn("walk error", "path", path, "error", err)
			s.emit(ScanEvent{Kind: EventWarn, Path: path, Message: err.Error()})
			return nil // Continue scanning
		}

		if !d.IsDir() {
			return nil
		}

		s.Logger.Debug("visiting directory", "path", path)

		for _, target := range s.Targets {
			matched, matchErr := evaluateIndicators(path, target)
			if matchErr != nil {
				s.Logger.Warn("indicator evaluation failed", "path", path, "target", target.Name, "error", matchErr)
				s.emit(ScanEvent{Kind: EventWarn, Path: path, Message: matchErr.Error()})
				continue
			}
			if !matched {
				continue
			}

			s.Logger.Debug("target matched", "path", path, "target", target.Name)

			// This directory matches a target — evaluate rules
			ruleResults := s.evaluateRules(path, target)
			results = append(results, ruleResults...)

			// Skip descending into this project directory
			s.Logger.Debug("skipping subtree", "path", path)
			return filepath.SkipDir
		}

		// No target matched this directory
		s.emit(ScanEvent{Kind: EventEnter, Path: path})

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

	var matched bool
	for _, rule := range target.Rules {
		ruleMatched, tag, err := rule.Evaluate(dirPath)
		if err != nil {
			s.Logger.Warn("rule evaluation failed", "path", dirPath, "target", target.Name, "error", err)
			continue
		}
		if !ruleMatched {
			s.Logger.Debug("rule not matched", "path", dirPath, "target", target.Name)
			continue
		}
		matched = true

		result := Result{
			Path:       dirPath,
			TargetName: target.Name,
			Tag:        tag,
		}

		if s.RemoveMode {
			if err := s.Tagger.Remove(dirPath, tag); err != nil {
				s.Logger.Warn("failed to remove tag", "tag", tag, "path", dirPath, "error", err)
				result.Action = "skipped"
			} else {
				s.Logger.Debug("tag removed", "tag", tag, "path", dirPath, "target", target.Name)
				result.Action = "untagged"
			}
		} else {
			if err := s.Tagger.Apply(dirPath, tag); err != nil {
				s.Logger.Warn("failed to apply tag", "tag", tag, "path", dirPath, "error", err)
				result.Action = "skipped"
			} else {
				s.Logger.Debug("tag applied", "tag", tag, "path", dirPath, "target", target.Name)
				result.Action = "tagged"
			}
		}

		s.emit(ScanEvent{Kind: EventMatch, Path: dirPath, TargetName: target.Name, Tag: tag, Action: result.Action})
		results = append(results, result)
	}

	if !matched {
		s.emit(ScanEvent{Kind: EventSkip, Path: dirPath, TargetName: target.Name})
	}

	return results
}
