package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExtensionIndicator_Match(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "track.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	ind := &FileExtensionIndicator{Extension: ".cpr"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("expected match for directory containing .cpr file")
	}
}

func TestFileExtensionIndicator_Mismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	ind := &FileExtensionIndicator{Extension: ".cpr"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected no match for directory without .cpr file")
	}
}

func TestFileExtensionIndicator_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	ind := &FileExtensionIndicator{Extension: ".cpr"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected no match for empty directory")
	}
}

func TestFileExtensionIndicator_IgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir.cpr"), 0755); err != nil {
		t.Fatal(err)
	}

	ind := &FileExtensionIndicator{Extension: ".cpr"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected no match for directory entry with extension")
	}
}

func TestFileExtensionIndicator_UnreadableDir(t *testing.T) {
	dir := t.TempDir()
	restricted := filepath.Join(dir, "restricted")
	if err := os.Mkdir(restricted, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(restricted, 0755) }()

	ind := &FileExtensionIndicator{Extension: ".cpr"}
	match, err := ind.IsMatch(restricted)
	if err == nil {
		t.Fatal("expected error for unreadable directory")
	}
	if match {
		t.Error("expected no match for unreadable directory")
	}
}

func TestFileExtensionIndicator_NonexistentDir(t *testing.T) {
	ind := &FileExtensionIndicator{Extension: ".cpr"}
	match, err := ind.IsMatch("/nonexistent/path")
	if err != nil {
		t.Errorf("expected nil error for nonexistent dir, got: %v", err)
	}
	if match {
		t.Error("expected no match for nonexistent directory")
	}
}

func TestDirectoryExtensionIndicator_Match(t *testing.T) {
	dir := t.TempDir()
	lunaDir := filepath.Join(dir, "Session.luna")
	if err := os.Mkdir(lunaDir, 0755); err != nil {
		t.Fatal(err)
	}

	ind := &DirectoryExtensionIndicator{Extension: ".luna"}
	match, err := ind.IsMatch(lunaDir)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("expected match for directory with .luna extension")
	}
}

func TestDirectoryExtensionIndicator_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	lunaFile := filepath.Join(dir, "Session.luna")
	if err := os.WriteFile(lunaFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	ind := &DirectoryExtensionIndicator{Extension: ".luna"}
	match, err := ind.IsMatch(lunaFile)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected no match for file (not directory) with extension")
	}
}

func TestDirectoryExtensionIndicator_NoExtension(t *testing.T) {
	dir := t.TempDir()

	ind := &DirectoryExtensionIndicator{Extension: ".luna"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected no match for directory without .luna extension")
	}
}

func TestDirectoryExtensionIndicator_NonexistentPath(t *testing.T) {
	ind := &DirectoryExtensionIndicator{Extension: ".luna"}
	match, err := ind.IsMatch("/nonexistent/Session.luna")
	if err != nil {
		t.Errorf("expected nil error for nonexistent path, got: %v", err)
	}
	if match {
		t.Error("expected no match for nonexistent path")
	}
}

func TestDirectoryExtensionIndicator_StatError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(blocker, "Session.luna")

	ind := &DirectoryExtensionIndicator{Extension: ".luna"}
	match, err := ind.IsMatch(childPath)
	if err == nil {
		t.Fatal("expected error for path through a file")
	}
	if match {
		t.Error("expected no match on error")
	}
}

func TestFileExistsIndicator_Match(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	ind := &FileExistsIndicator{FileName: "package.json"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("expected match when package.json file exists")
	}
}

func TestFileExistsIndicator_DirectoryWithSameName(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "package.json"), 0755); err != nil {
		t.Fatal(err)
	}

	ind := &FileExistsIndicator{FileName: "package.json"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected no match when package.json is a directory")
	}
}

func TestFileExistsIndicator_Missing(t *testing.T) {
	dir := t.TempDir()

	ind := &FileExistsIndicator{FileName: "package.json"}
	match, err := ind.IsMatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected no match when file doesn't exist")
	}
}

func TestFileExistsIndicator_NonexistentDir(t *testing.T) {
	ind := &FileExistsIndicator{FileName: "package.json"}
	match, err := ind.IsMatch("/nonexistent/path")
	if err != nil {
		t.Errorf("expected nil error for nonexistent dir, got: %v", err)
	}
	if match {
		t.Error("expected no match for nonexistent directory")
	}
}

func TestFileExistsIndicator_StatError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	ind := &FileExistsIndicator{FileName: "package.json"}
	match, err := ind.IsMatch(blocker)
	if err == nil {
		t.Fatal("expected error for stat through a file")
	}
	if match {
		t.Error("expected no match on error")
	}
}
