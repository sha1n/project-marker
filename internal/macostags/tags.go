//go:build darwin

package macostags

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation

#include <stdlib.h>
#include <string.h>
#import <Foundation/Foundation.h>

// Returns NULL on success, or a malloc'd localized description the caller must free.
// On failure, also populates outDomain (malloc'd, caller must free when non-NULL), outCode,
// and outHasPosix/outPosixCode with the NSPOSIXErrorDomain code from NSUnderlyingErrorKey when
// present, since Foundation reports the caller-facing errno there rather than in the top-level
// NSError for most Cocoa/OSStatus-domain failures.
static char *projmark_set_tag_names(const char *path, const char **tags, int count,
                                     char **outDomain, long long *outCode,
                                     int *outHasPosix, int *outPosixCode) {
	@autoreleasepool {
		*outDomain = NULL;
		*outCode = 0;
		*outHasPosix = 0;
		*outPosixCode = 0;

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
			*outDomain = strdup(error.domain.UTF8String);
			*outCode = (long long)error.code;

			NSError *underlying = error.userInfo[NSUnderlyingErrorKey];
			if ([underlying.domain isEqualToString:NSPOSIXErrorDomain]) {
				*outHasPosix = 1;
				*outPosixCode = (int)underlying.code;
			} else if ([error.domain isEqualToString:NSPOSIXErrorDomain]) {
				*outHasPosix = 1;
				*outPosixCode = (int)error.code;
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
	"io/fs"
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

	var cDomain *C.char
	var cCode C.longlong
	var cHasPosix, cPosixCode C.int

	cErr := C.projmark_set_tag_names(cPath, tagsPtr, C.int(len(cTags)), &cDomain, &cCode, &cHasPosix, &cPosixCode)
	if cErr == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(cErr))

	var domain string
	if cDomain != nil {
		defer C.free(unsafe.Pointer(cDomain))
		domain = C.GoString(cDomain)
	}
	description := C.GoString(cErr)
	c := causeFromNSError(domain, int64(cCode), cHasPosix != 0, int(cPosixCode), description)
	return fmt.Errorf("cannot update Finder tags on %q: %w", path, c)
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
// Tags stored in the legacy bare-name format or duplicated are rewritten so Finder colors them.
func AddTag(path, tag string) error {
	entries, err := readEntries(path)
	if err != nil {
		return err
	}

	names := tagNames(entries)
	if slices.Contains(names, tag) {
		if !needsRepair(entries, names) {
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
// bare-name or duplicate entries report false so that Apply rewrites them with color metadata.
func (t *Tagger) HasTag(path, tag string) (bool, error) {
	entries, err := readEntries(path)
	if err != nil {
		return false, err
	}
	names := tagNames(entries)
	return slices.Contains(names, tag) && !needsRepair(entries, names), nil
}

// HasTagOrder reports whether the given tags appear on path in the given relative order.
// Tags that are not present on path are ignored.
func (t *Tagger) HasTagOrder(path string, tags []string) (bool, error) {
	entries, err := readEntries(path)
	if err != nil {
		return false, err
	}
	names := tagNames(entries)
	return slices.Equal(names, reorderedNames(names, tags)), nil
}

// OrderTags rearranges the given tags within the positions they already occupy on path so
// they appear in the given relative order. Other tags keep their positions, and tags that
// are not present are ignored. Nothing is written when the order already holds.
func (t *Tagger) OrderTags(path string, tags []string) error {
	entries, err := readEntries(path)
	if err != nil {
		return err
	}
	names := tagNames(entries)
	reordered := reorderedNames(names, tags)
	if slices.Equal(names, reordered) {
		return nil
	}
	return SetTags(path, reordered)
}

// reorderedNames returns names with the subset also present in tags rearranged, in place, to
// follow tags' relative order; names not in tags, and tags not in names, keep their positions
// (or are ignored, respectively). Shared by HasTagOrder and OrderTags so the two cannot disagree
// on what "in order" means.
func reorderedNames(names, tags []string) []string {
	present := make([]string, 0, len(tags))
	for _, tag := range tags {
		if slices.Contains(names, tag) {
			present = append(present, tag)
		}
	}

	reordered := slices.Clone(names)
	next := 0
	for i, name := range names {
		if slices.Contains(present, name) {
			reordered[i] = present[next]
			next++
		}
	}
	return reordered
}

// readEntries is read directly from the xattr rather than through Foundation: it keeps
// per-directory scans cheap and exposes the raw entry format needed to detect legacy tags.
func readEntries(path string) ([]string, error) {
	size, err := unix.Getxattr(path, xattrKey, nil)
	if isAbsent(err) {
		return nil, nil
	}
	if err != nil {
		return nil, readError(path, err)
	}

	buf := make([]byte, size)
	n, err := unix.Getxattr(path, xattrKey, buf)
	if isAbsent(err) {
		return nil, nil
	}
	if err != nil {
		return nil, readError(path, err)
	}

	var entries []string
	if _, err := plist.Unmarshal(buf[:n], &entries); err != nil {
		return nil, fmt.Errorf("cannot read Finder tags on %q: the stored tags are malformed", path)
	}
	return entries, nil
}

// readError wraps a getxattr failure with the read-path message prefix and the shared cause
// mapping, so read and write failures for the same errno report identical reason text.
func readError(path string, err error) error {
	errno, _ := err.(unix.Errno)
	return fmt.Errorf("cannot read Finder tags on %q: %w", path, causeFromErrno(errno, err))
}

func isAbsent(err error) bool {
	return errors.Is(err, unix.ENOATTR) || errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR)
}

// cause is the friendly reason behind a tagging failure. It is shared by the getxattr read path
// and the Foundation write path so both report identical text for the same underlying errno, and
// so errors.Is matches fs.ErrNotExist, fs.ErrPermission, unix.EROFS, errors.ErrUnsupported, or the
// errno itself, per the mapping table in the design brief.
type cause struct {
	reason string
	errno  unix.Errno // 0 when the failure could not be mapped to a POSIX errno
}

func (c *cause) Error() string { return c.reason }

func (c *cause) Is(target error) bool {
	switch c.errno {
	case unix.ENOENT, unix.ENOTDIR:
		if target == fs.ErrNotExist {
			return true
		}
	case unix.EACCES, unix.EPERM:
		if target == fs.ErrPermission {
			return true
		}
	case unix.EROFS:
		if target == unix.EROFS {
			return true
		}
	case unix.ENOTSUP:
		if target == errors.ErrUnsupported {
			return true
		}
	}
	if errno, ok := target.(unix.Errno); ok {
		return c.errno != 0 && c.errno == errno
	}
	return false
}

// friendlyReason is the single source of truth for the mapping table's <reason> column. It
// falls back to description (the errno string or NSError localizedDescription) for causes that
// aren't in the table.
func friendlyReason(errno unix.Errno, description string) string {
	switch errno {
	case unix.ENOENT, unix.ENOTDIR:
		return "no such file or directory"
	case unix.EACCES, unix.EPERM:
		return "permission denied"
	case unix.EROFS:
		return "the volume is read-only"
	case unix.ENOTSUP:
		return "the file system does not support Finder tags"
	default:
		return description
	}
}

// causeFromErrno maps a getxattr failure to the shared cause vocabulary. err's own description
// (equal to errno.Error() when errno is known) is used verbatim for unmapped causes.
func causeFromErrno(errno unix.Errno, err error) error {
	return &cause{reason: friendlyReason(errno, err.Error()), errno: errno}
}

// causeFromNSError maps a Foundation NSError to the shared cause vocabulary. It prefers an
// underlying NSPOSIXErrorDomain errno (surfaced by the bridge via NSUnderlyingErrorKey) and
// otherwise falls back to the documented NSCocoaErrorDomain/NSOSStatusErrorDomain code crossovers,
// empirically confirmed for ENOENT (-43) and EACCES (-5000) against this macOS version.
func causeFromNSError(domain string, code int64, hasPosix bool, posixErrno int, description string) error {
	errno := errnoFromNSError(domain, code, hasPosix, posixErrno)
	return &cause{reason: friendlyReason(errno, description), errno: errno}
}

func errnoFromNSError(domain string, code int64, hasPosix bool, posixErrno int) unix.Errno {
	if hasPosix {
		return unix.Errno(posixErrno)
	}
	switch domain {
	case "NSPOSIXErrorDomain":
		return unix.Errno(code)
	case "NSCocoaErrorDomain":
		switch code {
		case 4, 260:
			return unix.ENOENT
		case 257, 513:
			return unix.EACCES
		case 642:
			return unix.EROFS
		}
	case "NSOSStatusErrorDomain":
		switch code {
		case -43, -120:
			return unix.ENOENT
		case -54, -61, -5000:
			return unix.EACCES
		case -44, -46:
			return unix.EROFS
		}
	}
	return 0
}

// tagNames de-duplicates because earlier versions could store a bare duplicate of a
// Finder-written tag, and the system tagging API persists duplicates as given.
func tagNames(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name, _, _ := strings.Cut(entry, entrySeparator)
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

func needsRepair(entries, names []string) bool {
	return len(names) != len(entries) || hasLegacyEntry(entries)
}

func hasLegacyEntry(entries []string) bool {
	return slices.ContainsFunc(entries, func(entry string) bool {
		return !strings.Contains(entry, entrySeparator)
	})
}
