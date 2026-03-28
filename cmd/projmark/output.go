package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sha1n/project-marker/internal/scanner"
)

const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)

func isTTY(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func shortestRelPath(path string, roots []string) string {
	rel := path
	for _, root := range roots {
		if r, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(r, "..") && len(r) < len(rel) {
			rel = r
		}
	}
	return rel
}

func verboseHandler(roots []string, w io.Writer, color bool) func(scanner.ScanEvent) {
	return func(e scanner.ScanEvent) {
		rel := shortestRelPath(e.Path, roots)

		// Skip the root directory itself
		if rel == "." {
			return
		}

		name := filepath.Base(rel)
		depth := strings.Count(rel, string(filepath.Separator))
		indent := strings.Repeat("  ", depth+1)

		switch e.Kind {
		case scanner.EventEnter:
			if color {
				_, _ = fmt.Fprintf(w, "%s%s◦ %s%s\n", indent, colorDim, name, colorReset)
			} else {
				_, _ = fmt.Fprintf(w, "%s◦ %s\n", indent, name)
			}

		case scanner.EventMatch:
			symbol, c := "●", colorGreen
			if e.Action == scanner.ActionUntagged {
				c = colorCyan
			}
			if e.Action == scanner.ActionAlreadyTagged {
				symbol = "="
				c = colorDim
			}
			if e.Action == scanner.ActionWouldTag || e.Action == scanner.ActionWouldUntag {
				symbol = "○"
				c = colorDim
			}
			detail := fmt.Sprintf("%s [%s]", e.TargetName, e.Tag)
			if color {
				_, _ = fmt.Fprintf(w, "%s%s%s %s  %s%s\n", indent, c, symbol, name, detail, colorReset)
			} else {
				_, _ = fmt.Fprintf(w, "%s%s %s  %s\n", indent, symbol, name, detail)
			}

		case scanner.EventSkip:
			detail := fmt.Sprintf("%s (no matching rule)", e.TargetName)
			if color {
				_, _ = fmt.Fprintf(w, "%s%s◦ %s  %s%s\n", indent, colorDim, name, detail, colorReset)
			} else {
				_, _ = fmt.Fprintf(w, "%s◦ %s  %s\n", indent, name, detail)
			}

		case scanner.EventWarn:
			if color {
				_, _ = fmt.Fprintf(w, "%s%s⚠ %s  %s%s\n", indent, colorYellow, name, e.Message, colorReset)
			} else {
				_, _ = fmt.Fprintf(w, "%s⚠ %s  %s\n", indent, name, e.Message)
			}

		default:
			// Unknown event kind — ignore gracefully.
		}
	}
}
