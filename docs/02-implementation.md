# Go CLI Migration Implementation Plan

## Goal Description
Convert the Python `tag_projects.py` script into a generic Go CLI application capable of identifying software/music projects (like Cubase and LUNA) dynamically based on an embedded YAML config, and applying macOS tags cleanly. The implementation will follow a strict Test-Driven Development (TDD) cycle, ensuring 100% coverage by design, and adopting a robust `Makefile` for builds, tests, and linting.

## Development Rules
Per the user's explicit request:
1. **Makefile**: Must leverage `mcp-acdc-server`'s Makefile structure (`make format`, `make lint`, `make test`, `make coverage`, `make build`).
2. **Commit Pipeline**: Every distinct task must be isolated. No task is considered "done" until tests exist, coverage is maximized, documentation is written, and it is committed to via `git`.
3. **Total Coverage**: Code must be designed to be testable (e.g., using interfaces for OS dependencies to allow mocking), ensuring close to 100% line coverage natively.

## Proposed Changes

### Configuration and Build
#### [NEW] [Makefile](file:///Users/shai/code/project-marker/Makefile)
A clone of the `mcp-acdc-server` makefile adapted for `projmark`.
#### [NEW] [go.mod](file:///Users/shai/code/project-marker/go.mod)
Go module file `github.com/sha1n/project-marker`.

### Core Engine
#### [NEW] [registry.go](file:///Users/shai/code/project-marker/internal/engine/registry.go)
Handles dynamic mapping of string IDs (from YAML) to concrete `Indicator` and `TagRule` structs.
#### [NEW] [registry_test.go](file:///Users/shai/code/project-marker/internal/engine/registry_test.go)
Validates all supported handlers map correctly to prevent silent failures.
#### [NEW] [indicators.go](file:///Users/shai/code/project-marker/internal/engine/indicators.go)
Implements FileExtension and DirectoryExtension matchers.
#### [NEW] [rules.go](file:///Users/shai/code/project-marker/internal/engine/rules.go)
Implements rules like `HasSubdirectoryRule`.

### Configuration
#### [NEW] [default.yaml](file:///Users/shai/code/project-marker/internal/config/default.yaml)
An embedded YAML defining Cubase/LUNA matching rules.
#### [NEW] [config.go](file:///Users/shai/code/project-marker/internal/config/config.go)
Logic parsing `default.yaml` into Go structs using `gopkg.in/yaml.v3`.

### Filesystem and OS Dependencies
#### [NEW] [tags.go](file:///Users/shai/code/project-marker/internal/macostags/tags.go)
Wraps Apple `xattr` toolset (`com.apple.metadata:_kMDItemUserTags`).
#### [NEW] [scanner.go](file:///Users/shai/code/project-marker/internal/scanner/scanner.go)
Main directory walking loop, tying config targets to engine validators.

### CLI 
#### [NEW] [main.go](file:///Users/shai/code/project-marker/cmd/projmark/main.go)
The `projmark` entrypoint handling flags (like `-r` for untagging).
#### [NEW] [main_test.go](file:///Users/shai/code/project-marker/cmd/projmark/main_test.go)
End-to-End mock workspace testing.

## Verification Plan
After each phase of implementation (Task 1 through Task 7):
1. **Formatting**: Run `make format`.
2. **Linting**: Run `make lint` (executing `golangci-lint run`).
3. **Tests & Coverage**: Run `make coverage`. Will view the generated coverage report/stdout to confirm lines covered matches expectations (targeting 100%).
4. **Build**: Run `make build` to ensure cross-platform (or at least `darwin/amd64` and `darwin/arm64`) binaries succeed.
5. **Commit**: `git add . && git commit -m "feat: [Task Description]"`
