# projmark

[![CI](https://github.com/sha1n/project-marker/actions/workflows/ci.yml/badge.svg)](https://github.com/sha1n/project-marker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sha1n/project-marker)](https://goreportcard.com/report/github.com/sha1n/project-marker)
[![GitHub release](https://img.shields.io/github/v/release/sha1n/project-marker)](https://github.com/sha1n/project-marker/releases/latest)

A command-line tool that scans directories and applies macOS Finder tags based on project type. Designed for music production workflows, projmark identifies Cubase and LUNA projects and tags directories that contain exported or mixed-down content, making it easy to see at a glance which projects have final outputs.

## Installation

### Homebrew

```bash
brew tap sha1n/tap
brew install projmark
```

### GitHub Releases

Download the latest binary for your platform from the [Releases](https://github.com/sha1n/project-marker/releases/latest) page.

### From Source

```bash
git clone https://github.com/sha1n/project-marker.git
cd project-marker
make build
```

The compiled binaries will be placed in the `bin/` directory.

## Usage

Scan a directory and tag matching projects:

```bash
projmark ~/Music/Projects
```

Remove previously applied tags:

```bash
projmark -r ~/Music/Projects
```

Scan multiple directories at once:

```bash
projmark ~/Music/Cubase ~/Music/LUNA
```

## Options

| Flag | Description |
|------|-------------|
| `-r` | Remove tags instead of adding them |
| `-version` | Print version information |
| `--completion-bash` | Output bash completion script |
| `--completion-zsh` | Output zsh completion script |
| `--completion-fish` | Output fish completion script |

## Shell Completion

### Bash

```bash
eval "$(projmark --completion-bash)"
```

To make it permanent, add the line above to your `~/.bashrc` or `~/.bash_profile`.

### Zsh

```bash
eval "$(projmark --completion-zsh)"
```

To make it permanent, add the line above to your `~/.zshrc`.

### Fish

```fish
projmark --completion-fish | source
```

To make it permanent:

```fish
projmark --completion-fish > ~/.config/fish/completions/projmark.fish
```

## Configuration

projmark ships with a built-in configuration that defines which project types to detect and how to tag them. The default configuration is embedded at build time from `internal/config/default.yaml`:

```yaml
targets:
  - name: "Cubase"
    indicators:
      - type: "file_extension"
        value: ".cpr"
    rules:
      - type: "has_subdirectory"
        match: "any"
        value:
          - "Mixdown"
        apply_tag: "Blue"

  - name: "LUNA"
    indicators:
      - type: "directory_extension"
        value: ".luna"
    rules:
      - type: "has_subdirectory"
        match: "any"
        value:
          - "Exported Files"
        apply_tag: "Blue"
```

Each target type is defined by:

- **name** -- a human-readable label for the project type.
- **indicators** -- patterns used to identify a project directory. `file_extension` matches files within the directory, while `directory_extension` matches the directory name itself.
- **rules** -- conditions that must be met for a tag to be applied. The `has_subdirectory` rule checks for the presence of specific subdirectories (e.g., `Mixdown` or `Exported Files`).
- **apply_tag** -- the macOS Finder tag to apply. Supported tag names include `Red`, `Orange`, `Yellow`, `Green`, `Blue`, `Purple`, and `Gray`.

## How It Works

1. projmark walks each provided directory, inspecting immediate subdirectories.
2. For each subdirectory, it checks whether any configured target indicators match (e.g., a `.cpr` file for Cubase projects).
3. When a match is found, it evaluates the target's rules to determine if the directory qualifies for tagging (e.g., contains a `Mixdown` folder).
4. Qualifying directories receive the configured macOS Finder tag via extended attributes.
5. In remove mode (`-r`), the same detection logic runs but tags are removed instead of applied.

## Development

| Command | Description |
|---------|-------------|
| `make build` | Build binaries for all supported platforms |
| `make test` | Run all tests |
| `make lint` | Run all linters (go vet, golangci-lint, format check) |
| `make format` | Format Go source files |
| `make coverage` | Run tests with coverage report |
| `make coverage-html` | Run tests and open coverage report in browser |
| `make release` | Create a release with GoReleaser |
| `make clean` | Remove build artifacts |
| `make install` | Check and install missing dependencies |
