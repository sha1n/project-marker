package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/sha1n/project-marker/internal/config"
	"github.com/sha1n/project-marker/internal/engine"
	"github.com/sha1n/project-marker/internal/macostags"
	"github.com/sha1n/project-marker/internal/scanner"
)

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
	removeMode := fs.Bool("r", false, "Remove tags instead of adding them")
	version := fs.Bool("version", false, "Print version information")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *version {
		fmt.Printf("%s %s (build: %s)\n", ProgramName, Version, Build)
		return 0
	}

	dirs := fs.Args()
	if len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-r] <directory> [directory...]\n", ProgramName)
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -r\tRemove tags instead of adding them\n")
		return 1
	}

	// Validate directories
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: %s is not a valid directory\n", dir)
			return 1
		}
	}

	// Load configuration
	registry := engine.NewRegistry()
	targets, err := config.Load(registry)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
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
		log.Fatalf("scan error: %v", err)
	}

	for _, r := range results {
		switch r.Action {
		case "tagged":
			fmt.Printf("  ✓ Tagged [%s] %s (%s)\n", r.Tag, r.Path, r.TargetName)
		case "untagged":
			fmt.Printf("  ✓ Untagged [%s] %s (%s)\n", r.Tag, r.Path, r.TargetName)
		case "skipped":
			fmt.Printf("  ✗ Skipped %s (%s)\n", r.Path, r.TargetName)
		}
	}

	actionWord := "Tagged"
	if *removeMode {
		actionWord = "Untagged"
	}
	fmt.Printf("\nComplete! %s %d director%s.\n", actionWord, len(results), pluralize(len(results)))

	return 0
}

func pluralize(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
