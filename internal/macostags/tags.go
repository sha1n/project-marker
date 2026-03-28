//go:build darwin

package macostags

import (
	"fmt"

	"golang.org/x/sys/unix"
	"howett.net/plist"
)

const xattrKey = "com.apple.metadata:_kMDItemUserTags"

// SetTags sets the Finder tags on a file or directory, replacing any existing tags.
func SetTags(path string, tags []string) error {
	data, err := plist.Marshal(tags, plist.BinaryFormat)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	if err := unix.Setxattr(path, xattrKey, data, 0); err != nil {
		return fmt.Errorf("setxattr %s: %w", path, err)
	}
	return nil
}

// GetTags reads the Finder tags from a file or directory.
func GetTags(path string) ([]string, error) {
	// First get the size of the attribute
	size, err := unix.Getxattr(path, xattrKey, nil)
	if err != nil {
		return nil, nil // No tags
	}

	buf := make([]byte, size)
	_, err = unix.Getxattr(path, xattrKey, buf)
	if err != nil {
		return nil, fmt.Errorf("getxattr %s: %w", path, err)
	}

	var tags []string
	if _, err := plist.Unmarshal(buf, &tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	return tags, nil
}

// AddTag adds a tag to existing tags without removing others.
func AddTag(path, tag string) error {
	existing, err := GetTags(path)
	if err != nil {
		return err
	}

	for _, t := range existing {
		if t == tag {
			return nil // Already tagged
		}
	}

	existing = append(existing, tag)
	return SetTags(path, existing)
}

// RemoveTag removes a specific tag from a file or directory.
func RemoveTag(path, tag string) error {
	existing, err := GetTags(path)
	if err != nil {
		return err
	}

	filtered := make([]string, 0, len(existing))
	found := false
	for _, t := range existing {
		if t == tag {
			found = true
		} else {
			filtered = append(filtered, t)
		}
	}

	if !found {
		return nil
	}

	if len(filtered) == 0 {
		if err := unix.Removexattr(path, xattrKey); err != nil {
			return fmt.Errorf("removexattr %s: %w", path, err)
		}
		return nil
	}

	return SetTags(path, filtered)
}

// Tagger implements the scanner.Tagger interface for macOS.
type Tagger struct{}

// Apply adds a tag to the given path.
func (t *Tagger) Apply(path, tag string) error {
	return AddTag(path, tag)
}

// Remove removes a tag from the given path.
func (t *Tagger) Remove(path, tag string) error {
	return RemoveTag(path, tag)
}

// HasTag checks whether the given tag is present on the path.
func (t *Tagger) HasTag(path, tag string) (bool, error) {
	tags, err := GetTags(path)
	if err != nil {
		return false, err
	}
	for _, existing := range tags {
		if existing == tag {
			return true, nil
		}
	}
	return false, nil
}
