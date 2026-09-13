//go:build darwin && !cgo

package macostags

// tags.go declares `import "C"`, so the go tool silently drops it from the build whenever cgo
// is disabled — leaving this package with no Tagger, GetTags, or SetTags on darwin and surfacing
// as a baffling "undefined: macostags.Tagger" far from the actual cause. Fail the build here
// instead, with an identifier that states the fix.
var _ = projmark_requires_cgo_on_darwin__set_CGO_ENABLED_1
