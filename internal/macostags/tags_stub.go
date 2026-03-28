//go:build !darwin

package macostags

import "errors"

// ErrUnsupportedPlatform is returned when macOS tag operations are called on non-darwin platforms.
var ErrUnsupportedPlatform = errors.New("macOS tags are only supported on darwin")

func SetTags(path string, tags []string) error { return ErrUnsupportedPlatform }
func GetTags(path string) ([]string, error)    { return nil, ErrUnsupportedPlatform }
func AddTag(path, tag string) error            { return ErrUnsupportedPlatform }
func RemoveTag(path, tag string) error         { return ErrUnsupportedPlatform }

// Tagger implements the scanner.Tagger interface as a no-op on non-darwin platforms.
type Tagger struct{}

func (t *Tagger) Apply(path, tag string) error          { return ErrUnsupportedPlatform }
func (t *Tagger) Remove(path, tag string) error         { return ErrUnsupportedPlatform }
func (t *Tagger) HasTag(path, tag string) (bool, error) { return false, ErrUnsupportedPlatform }
