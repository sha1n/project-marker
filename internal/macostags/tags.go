//go:build darwin

package macostags

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation

#include <stdlib.h>
#include <string.h>
#import <Foundation/Foundation.h>

// Returns NULL on success, or a malloc'd error message the caller must free.
static char *projmark_set_tag_names(const char *path, const char **tags, int count) {
	@autoreleasepool {
		NSString *nsPath = [NSString stringWithUTF8String:path];
		if (nsPath == nil) {
			return strdup("path is not valid UTF-8");
		}
		NSMutableArray<NSString *> *names = [NSMutableArray arrayWithCapacity:count];
		for (int i = 0; i < count; i++) {
			NSString *name = [NSString stringWithUTF8String:tags[i]];
			if (name == nil) {
				return strdup("tag name is not valid UTF-8");
			}
			[names addObject:name];
		}
		NSURL *url = [NSURL fileURLWithPath:nsPath];
		NSError *error = nil;
		if (![url setResourceValue:names forKey:NSURLTagNamesKey error:&error]) {
			if (error == nil) {
				return strdup("unknown error");
			}
			return strdup(error.localizedDescription.UTF8String);
		}
		return NULL;
	}
}
*/
import "C"

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
	"howett.net/plist"
)

const xattrKey = "com.apple.metadata:_kMDItemUserTags"

// entrySeparator splits a stored tag entry into its name and Finder color index ("Name\n<color>").
const entrySeparator = "\n"

// SetTags replaces all Finder tags on a file or directory through the system tagging API,
// which also maintains the Finder label color. An empty tags slice removes all tags.
func SetTags(path string, tags []string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cTags := make([]*C.char, len(tags))
	for i, tag := range tags {
		cTags[i] = C.CString(tag)
	}
	defer func() {
		for _, cTag := range cTags {
			C.free(unsafe.Pointer(cTag))
		}
	}()

	var tagsPtr **C.char
	if len(cTags) > 0 {
		tagsPtr = &cTags[0]
	}

	if cErr := C.projmark_set_tag_names(cPath, tagsPtr, C.int(len(cTags))); cErr != nil {
		defer C.free(unsafe.Pointer(cErr))
		return fmt.Errorf("set tags %s: %s", path, C.GoString(cErr))
	}
	return nil
}

// GetTags reads the Finder tag names from a file or directory in stored order.
// It returns nil when the path has no tags or does not exist.
func GetTags(path string) ([]string, error) {
	entries, err := readEntries(path)
	if err != nil {
		return nil, err
	}
	return tagNames(entries), nil
}

// AddTag adds a tag to existing tags without removing others.
// Tags stored in the legacy bare-name format are rewritten so Finder colors them.
func AddTag(path, tag string) error {
	entries, err := readEntries(path)
	if err != nil {
		return err
	}

	names := tagNames(entries)
	if slices.Contains(names, tag) {
		if !hasLegacyEntry(entries) {
			return nil
		}
		return SetTags(path, names)
	}
	return SetTags(path, append(names, tag))
}

// RemoveTag removes a specific tag from a file or directory.
func RemoveTag(path, tag string) error {
	names, err := GetTags(path)
	if err != nil {
		return err
	}
	if !slices.Contains(names, tag) {
		return nil
	}
	return SetTags(path, slices.DeleteFunc(names, func(name string) bool { return name == tag }))
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

// HasTag reports whether the given tag is present on the path. Paths carrying legacy
// bare-name entries report false so that Apply rewrites them with color metadata.
func (t *Tagger) HasTag(path, tag string) (bool, error) {
	entries, err := readEntries(path)
	if err != nil {
		return false, err
	}
	return slices.Contains(tagNames(entries), tag) && !hasLegacyEntry(entries), nil
}

// readEntries is read directly from the xattr rather than through Foundation: it keeps
// per-directory scans cheap and exposes the raw entry format needed to detect legacy tags.
func readEntries(path string) ([]string, error) {
	size, err := unix.Getxattr(path, xattrKey, nil)
	if isAbsent(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getxattr %s: %w", path, err)
	}

	buf := make([]byte, size)
	n, err := unix.Getxattr(path, xattrKey, buf)
	if isAbsent(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getxattr %s: %w", path, err)
	}

	var entries []string
	if _, err := plist.Unmarshal(buf[:n], &entries); err != nil {
		return nil, fmt.Errorf("unmarshal tags %s: %w", path, err)
	}
	return entries, nil
}

func isAbsent(err error) bool {
	return errors.Is(err, unix.ENOATTR) || errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR)
}

func tagNames(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i], _, _ = strings.Cut(entry, entrySeparator)
	}
	return names
}

func hasLegacyEntry(entries []string) bool {
	return slices.ContainsFunc(entries, func(entry string) bool {
		return !strings.Contains(entry, entrySeparator)
	})
}
