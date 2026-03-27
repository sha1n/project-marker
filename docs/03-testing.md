# Test Plan: Generic Project Marker

## 1. Overview
This test plan validates all functionality of the `projmark` CLI. It guarantees correct file-system traversal, accurate target identification based on dynamic configuration handlers, reliable tagging on macOS via `xattr`, and comprehensive edge case handling.

## 2. Static Handler Verification (The Registry)
To ensure the dynamic configuration maps safely to actual implementations, all indicator and rule types must be represented by static string constants in the code (e.g., `engine.IndicatorFileExtension`, `engine.RuleHasSubdirectory`). 

**Test Case**: `TestRegistryExhaustiveness`
* **Objective**: Guarantee that every supported handler capability defined in the code maps to a registered, working implementation.
* **Mechanism**: The test iterates over a statically defined slice or map of all known handler constants (e.g., `engine.AllSupportedIndicators()`). For each, it invokes the registry to retrieve the factory function. If any constant fails to resolve, or if invoking the factory panics, the test fails.
* **Verification**: A secondary step in the test ensures that the embedded YAML configuration *only* contains references to these statically verified handler names.

## 3. Unit Tests per Component

### 3.1 `internal/engine` (Indicators and Rules)
**Objective**: Test the isolated logic of every condition handler using mock filesystems (`t.TempDir()` or `testing/fstest`).

* **`FileExtensionIndicator`**:
  * *Match*: Directory contains `test.cpr`. (Pass)
  * *Mismatch*: Directory contains only `test.txt`. (Fail)
  * *Edge Case*: Directory is totally empty. (Fail)
  * *Edge Case*: Simulated permission denied when trying to read directory contents. (Handles gracefully, returns false).
* **`DirectoryExtensionIndicator`**:
  * *Match*: Path evaluates to a directory named `Session.luna`.
  * *Edge Case*: A standard *file* named `Session.luna` exists instead of a directory. (Fail)
* **`FileExistsIndicator`**:
  * *Match*: Exact filename exists (e.g., `package.json`).
  * *Edge Case*: Target is a directory coincidentally named `package.json`. (Fail)
* **`HasSubdirectoryRule`**:
  * *Match*: Root directory contains all requested subdirectories.
  * *Mismatch*: Contains only a subset (e.g., has "Mixdown" but missing "Exported").
  * *Edge Case*: The subdirectory "Mixdown" is actually a flat file. (Fail)

### 3.2 `internal/config` (Configuration Loader)
**Objective**: Verify the embedded configuration loads predictably and catches user mapping errors early.
* *Valid Config*: Parse the built-in `default.yaml` and verify structs populate accurately.
* *Edge Case*: Malformed YAML returns a descriptive error immediately on boot.
* *Edge Case*: If the YAML requests a rule type (e.g., `has_file_pattern`) that isn't defined in the static constants, parsing must instantly fail rather than failing silently during a directory sweep.

### 3.3 `internal/macostags` (macOS Integration)
**Objective**: Verify system-level `xattr` interactions for adding/removing Finder tags (`com.apple.metadata:_kMDItemUserTags`).
* *Write Tag*: Create a temp file, serialize the binary plist data, apply the extended attribute, read it back, and assert the tags match exactly.
* *Remove Tag*: Verify the attribute is successfully deleted when explicitly asked.
* *Edge Case*: Attempting to read/write tags on a missing file returns a predictable, non-panic error.
* *Edge Case*: Accessing files on a read-only filesystem or isolated sandbox triggers `permission denied`, which should be captured and logged as skipped rather than crashing the program.

### 3.4 `internal/scanner` (Directory Traversal)
**Objective**: Ensure the `filepath.WalkDir` logic executes rules efficiently and respects OS boundaries.
* *Traversal Pathing*: Given a mock tree with a generic directory layer and two project directories deep inside, verify the scanner finds both.
* *Optimization Check*: Assert that once a directory is successfully flagged as a project root (e.g., Cubase project), the scanner skips traversing all its child directories (`filepath.SkipDir`), preventing slow loops inside heavy node_modules or sample libraries.
* *Edge Case (Unreadable Dirs)*: Place a directory with `000` permissions mid-tree. The scanner must log standard Go `fs.ErrPermission` and continue scanning sibling directories completely unhindered.
* *Edge Case (Symlinks)*: Explicitly confirm that by using `WalkDir`, symlinks are ignored, preventing cyclical recursion crashes.

## 4. End-to-End (E2E) Integration Tests
**Objective**: Run the compiled CLI boundary exactly as a user will, orchestrating all components.

1. **Setup**: Generate a mock OS workspace in `t.TempDir()` mimicking a real drive surface:
   * `/Users/fake/Music/Track1/Track1.cpr`
   * `/Users/fake/Music/Track1/Mixdown/audio.wav`
   * `/Users/fake/Music/Track2/Track2.cpr` *(No mixdown folder)*
   * `/Users/fake/Code/App/package.json`
2. **Execute Tag**: Invoke `projmark /Users/fake`.
   * Assert `Track1` directory successfully applies tag "Blue".
   * Assert `Track2` directory remains untagged due to failed rule.
3. **Execute Untag**: Invoke `projmark -r /Users/fake`.
   * Assert `Track1` directory had its "Blue" tag successfully removed.
