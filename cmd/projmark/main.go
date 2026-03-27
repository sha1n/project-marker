package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sha1n/project-marker/internal/config"
	"github.com/sha1n/project-marker/internal/engine"
	"github.com/sha1n/project-marker/internal/macostags"
	"github.com/sha1n/project-marker/internal/scanner"
)

//go:embed completions/projmark.bash
var bashCompletion string

//go:embed completions/projmark.zsh
var zshCompletion string

//go:embed completions/projmark.fish
var fishCompletion string

var (
	Version     = "dev"
	Build       = "HEAD"
	ProgramName = "projmark"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet(ProgramName, flag.ContinueOnError)
	fs.Usage = func() { printUsage(os.Stderr) }

	removeMode := fs.Bool("r", false, "Remove tags instead of adding them")
	version := fs.Bool("version", false, "Print version information")
	completionBash := fs.Bool("completion-bash", false, "Output bash completion script")
	completionZsh := fs.Bool("completion-zsh", false, "Output zsh completion script")
	completionFish := fs.Bool("completion-fish", false, "Output fish completion script")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if *version {
		fmt.Printf("%s %s (build: %s)\n", ProgramName, Version, Build)
		return 0
	}

	if *completionBash {
		fmt.Print(bashCompletion)
		return 0
	}
	if *completionZsh {
		fmt.Print(zshCompletion)
		return 0
	}
	if *completionFish {
		fmt.Print(fishCompletion)
		return 0
	}

	dirs := fs.Args()
	if len(dirs) == 0 {
		printUsage(os.Stderr)
		return 1
	}

	// Validate directories
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				if _, lstatErr := os.Lstat(dir); lstatErr == nil {
					fmt.Fprintf(os.Stderr, "Error: %s is a symlink to a non-existent target\n", dir)
				} else {
					fmt.Fprintf(os.Stderr, "Error: %s does not exist\n", dir)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s: %v\n", dir, err)
			}
			return 1
		}
		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", dir)
			return 1
		}
	}

	// Load configuration
	registry := engine.NewRegistry()
	targets, err := config.Load(registry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
		return 1
	}

	// Run scanner
	s := &scanner.Scanner{
		Targets:    targets,
		Tagger:     &macostags.Tagger{},
		RemoveMode: *removeMode,
	}

	action := "Scanning"
	if *removeMode {
		action = "Removing tags from"
	}

	for _, dir := range dirs {
		fmt.Printf("%s: %s\n", action, dir)
	}

	results, err := s.Scan(dirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: scan failed: %v\n", err)
		return 1
	}

	var actionedCount int
	for _, r := range results {
		switch r.Action {
		case "tagged":
			fmt.Printf("  ✓ Tagged [%s] %s (%s)\n", r.Tag, r.Path, r.TargetName)
			actionedCount++
		case "untagged":
			fmt.Printf("  ✓ Untagged [%s] %s (%s)\n", r.Tag, r.Path, r.TargetName)
			actionedCount++
		case "skipped":
			fmt.Printf("  ✗ Skipped %s (%s)\n", r.Path, r.TargetName)
		}
	}

	actionWord := "Tagged"
	if *removeMode {
		actionWord = "Untagged"
	}
	skippedCount := len(results) - actionedCount
	fmt.Printf("\nComplete! %s %d director%s", actionWord, actionedCount, pluralize(actionedCount))
	if skippedCount > 0 {
		fmt.Printf(" (%d skipped)", skippedCount)
	}
	fmt.Println(".")

	return 0
}

func printUsage(w io.Writer) {
	const usageTemplate = `Usage: {{name}} [-r] <directory> [directory...]

Scan directories and apply macOS Finder tags based on project type.
{{name}} identifies music production projects (Cubase, LUNA) and tags
directories that contain exported/mixed-down content.

Options:
  -r                    Remove tags instead of adding them
  --version             Print version information
  --completion-bash     Output bash completion script
  --completion-zsh      Output zsh completion script
  --completion-fish     Output fish completion script

Examples:
  {{name}} ~/Music/Projects
  {{name}} -r ~/Music/Projects
  {{name}} ~/Music/Cubase ~/Music/LUNA

Shell Completion:
  Bash:  eval "$({{name}} --completion-bash)"
  Zsh:   eval "$({{name}} --completion-zsh)"
  Fish:  {{name}} --completion-fish | source
`
	_, _ = strings.NewReplacer("{{name}}", ProgramName).WriteString(w, usageTemplate)
}

func pluralize(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
