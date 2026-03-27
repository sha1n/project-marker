# Go CLI Migration Plan: Generic Project Marker

## 1. Overview
The goal is to convert the Python-based `tag_projects.py` script into a fast, portable, and generic Go CLI application. The application will scan specified directories and identify target directories (projects, workspaces, etc.) based on customizable indicators. It will then apply macOS Finder tags based on dynamic rules defined in an embedded YAML configuration file.

## 2. Key Requirements
* **Input**: Accept a directory or a list of directories as positional arguments.
* **Scanning**: Recursively walk the directory tree to identify "target directories" (e.g., Cubase projects, Node.js projects, photo albums).
* **Dynamic Resolution (The Engine)**: The system must dynamically resolve indicator types (how to spot a project) and rule types (when and how to tag it) based on the configuration string, utilizing a Handler Registry pattern.
* **Marking Logic**: For each identified project, evaluate the tag rules (e.g., "does it have a Mixdown folder?") and apply macOS extended attribute tags (e.g., "Blue") to the project root.
* **Configuration**: Use an embedded YAML configuration file to define rules. This keeps the core engine fully generic while mapping specific logic (like DAW workflows) in human-readable config.

## 3. Project Structure

```text
project-marker/
├── cmd/
│   └── projmark/
│       └── main.go           # CLI application entry point
├── internal/
│   ├── config/
│   │   ├── config.go         # Structs and logic to load embedded YAML
│   │   └── default.yaml      # Embedded YAML configuration file
│   ├── engine/
│   │   ├── registry.go       # Dynamically registers and resolves indicators/rules
│   │   ├── indicators.go     # Indicator implementations (e.g., FileExtensionIndicator)
│   │   └── rules.go          # Rule implementations (e.g., SubdirectoryPresenceRule)
│   ├── scanner/
│   │   └── scanner.go        # Directory walking using injected indicators & rules
│   └── macostags/
│       └── tags.go           # Wrapper for Apple xattr logic (add/remove tags)
├── go.mod
└── go.sum
```

## 4. Generic Configuration Design & Registry
By using a Handler Registry pattern, the `engine` package can dynamically load the correct Go structs based on the `type` field in the YAML.

```yaml
# internal/config/default.yaml
targets:
  - name: "Cubase"
    indicators:
      - type: "file_extension"
        value: ".cpr"
    rules:
      - type: "has_subdirectory"
        value: ["Mixdown", "Exported Files"]
        apply_tag: "Blue"

  - name: "LUNA"
    indicators:
      - type: "directory_extension"
        value: ".luna"
    rules:
      - type: "has_subdirectory"
        value: ["Mixdown", "Exported Files"]
        apply_tag: "Blue"
        
  # Example of extensibility for other workflows:
  - name: "Node.js"
    indicators:
      - type: "file_exists"
        value: "package.json"
    rules:
      - type: "has_directory"
        value: ["node_modules"]
        apply_tag: "Orange"
```

### Handler Interface Example
```go
type Indicator interface {
    // IsMatch returns true if the directory looks like the target project
    IsMatch(dirPath string) (bool, error)
}

type TagRule interface {
    // Evaluate returns true and the tag to apply if the condition is met
    Evaluate(dirPath string) (bool, string, error) 
}
```
The `registry` maps a string (e.g., `"file_extension"`) to a factory function that returns an `Indicator` or `TagRule` implementation. This ensures the CLI tool is entirely generic and extensible.

## 5. Implementation Steps

### Phase 1: Skeleton & Configuration Engine
1. Initialize the Go module (e.g., `go mod init github.com/sha1n/project-marker`).
2. Implement `Indicator` and `TagRule` interfaces and the Registration Engine (`internal/engine`).
3. Write standard handlers (`file_extension`, `file_exists`, `directory_extension`, `has_subdirectory`).
4. Implement the `internal/config` package using `//go:embed` to read the YAML, map the types, and instantiate handlers from the registry.

### Phase 2: macOS Tags Interface
1. Build `internal/macostags`, leveraging the OS-level `xattr` system for adding and removing macOS binary plist tags (`com.apple.metadata:_kMDItemUserTags`).

### Phase 3: Scanner Execution
1. Implement the `internal/scanner` to use `filepath.WalkDir`. It will evaluate every visited directory against configured target `indicators`.
2. If an indicator matches, run the target's associated `rules` and invoke the `macostags` package if conditions are met. Ensure we skip walking deep into directories we've already identified as projects to save time.

### Phase 4: CLI Interface
1. Write the main entry point to support positional arguments (directories) and flags (e.g., `-r` to explicitly untag).
2. We'll use `projmark` for the binary name, or `tag-projects` if you prefer parity with the old Python script.

### Phase 5: Build & Distribution
1. Ensure a simple `Makefile` or `go build` command yielding a standalone binary.
