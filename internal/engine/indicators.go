package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileExtensionIndicator matches directories containing an entry (file or directory) with the given extension.
type FileExtensionIndicator struct {
	Extension string
}

// NewFileExtensionIndicator creates a FileExtensionIndicator.
func NewFileExtensionIndicator(value string) (Indicator, error) {
	return &FileExtensionIndicator{Extension: value}, nil
}

func (i *FileExtensionIndicator) IsMatch(dirPath string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading directory %s: %w", dirPath, err)
	}
	for _, entry := range entries {
		if strings.EqualFold(filepath.Ext(entry.Name()), i.Extension) {
			return true, nil
		}
	}
	return false, nil
}

// DirectoryExtensionIndicator matches directories whose own name has the given extension.
type DirectoryExtensionIndicator struct {
	Extension string
}

// NewDirectoryExtensionIndicator creates a DirectoryExtensionIndicator.
func NewDirectoryExtensionIndicator(value string) (Indicator, error) {
	return &DirectoryExtensionIndicator{Extension: value}, nil
}

func (i *DirectoryExtensionIndicator) IsMatch(dirPath string) (bool, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", dirPath, err)
	}
	if !info.IsDir() {
		return false, nil
	}
	return strings.EqualFold(filepath.Ext(info.Name()), i.Extension), nil
}

// FileExistsIndicator matches directories containing a specific entry (file or directory).
type FileExistsIndicator struct {
	FileName string
}

// NewFileExistsIndicator creates a FileExistsIndicator.
func NewFileExistsIndicator(value string) (Indicator, error) {
	return &FileExistsIndicator{FileName: value}, nil
}

func (i *FileExistsIndicator) IsMatch(dirPath string) (bool, error) {
	_, err := os.Stat(filepath.Join(dirPath, i.FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", filepath.Join(dirPath, i.FileName), err)
	}
	return true, nil
}
