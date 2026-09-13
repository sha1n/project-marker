package scanner

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sha1n/project-marker/internal/config"
	"github.com/sha1n/project-marker/internal/engine"
)

// mockTagger records Apply/Remove calls for assertions.
// It does NOT implement TagChecker, so it tests the fallback path.
type mockTagger struct {
	applied []tagCall
	removed []tagCall
}

type tagCall struct {
	path string
	tag  string
}

func (m *mockTagger) Apply(path, tag string) error {
	m.applied = append(m.applied, tagCall{path, tag})
	return nil
}

func (m *mockTagger) Remove(path, tag string) error {
	m.removed = append(m.removed, tagCall{path, tag})
	return nil
}

// mockTagCheckerTagger implements both Tagger and TagChecker.
type mockTagCheckerTagger struct {
	mockTagger
	tags map[string]map[string]bool // path -> tag -> present
}

func newMockTagCheckerTagger() *mockTagCheckerTagger {
	return &mockTagCheckerTagger{tags: make(map[string]map[string]bool)}
}

func (m *mockTagCheckerTagger) setTag(path, tag string) {
	if m.tags[path] == nil {
		m.tags[path] = make(map[string]bool)
	}
	m.tags[path][tag] = true
}

func (m *mockTagCheckerTagger) HasTag(path, tag string) (bool, error) {
	if m.tags[path] != nil {
		return m.tags[path][tag], nil
	}
	return false, nil
}

// failingTagCheckerTagger implements TagChecker but always returns an error from HasTag.
type failingTagCheckerTagger struct {
	mockTagger
	hasTagErr error
}

func (m *failingTagCheckerTagger) HasTag(path, tag string) (bool, error) {
	return false, m.hasTagErr
}

// mockRepairCheckerTagger implements Tagger, TagChecker and TagRepairChecker (NeedsRepair).
// It embeds mockTagCheckerTagger for Apply/Remove/HasTag/setTag.
type mockRepairCheckerTagger struct {
	mockTagCheckerTagger
	needsRepair      map[string]bool // path -> needs repair
	needsRepairErr   error
	needsRepairCalls int
}

func newMockRepairCheckerTagger() *mockRepairCheckerTagger {
	return &mockRepairCheckerTagger{
		mockTagCheckerTagger: *newMockTagCheckerTagger(),
		needsRepair:          make(map[string]bool),
	}
}

func (m *mockRepairCheckerTagger) NeedsRepair(path string) (bool, error) {
	m.needsRepairCalls++
	if m.needsRepairErr != nil {
		return false, m.needsRepairErr
	}
	return m.needsRepair[path], nil
}

// errorIndicator always returns an error from IsMatch.
type errorIndicator struct{ err error }

func (e *errorIndicator) IsMatch(string) (bool, error) { return false, e.err }

// errorRule always returns an error from Evaluate.
type errorRule struct{ err error }

func (e *errorRule) Evaluate(string) (bool, string, error) { return false, "", e.err }

// mockTaggerWithChecker embeds mockTagger and adds TagChecker support.
type mockTaggerWithChecker struct {
	mockTagger
	hasTag    map[string]bool // key: "path\x00tag"
	hasTagErr error
}

func (m *mockTaggerWithChecker) HasTag(path, tag string) (bool, error) {
	if m.hasTagErr != nil {
		return false, m.hasTagErr
	}
	return m.hasTag[path+"\x00"+tag], nil
}

// failTagger returns configured errors from Apply/Remove.
type failTagger struct {
	applyErr  error
	removeErr error
}

func (f *failTagger) Apply(_, _ string) error  { return f.applyErr }
func (f *failTagger) Remove(_, _ string) error { return f.removeErr }

// orderCall records a single OrderTags invocation.
type orderCall struct {
	path string
	tags []string
}

// mockTagOrderer implements Tagger, TagChecker and the TagOrderer contract:
// HasTagOrder/OrderTags. It embeds mockTagCheckerTagger for
// Apply/Remove/HasTag, and lets a single tag's Apply be forced to fail so
// tests can produce a directory with only one eligible (successfully
// applied) tag.
type mockTagOrderer struct {
	mockTagCheckerTagger
	failApplyTag string

	hasTagOrderCalls  int
	hasTagOrderResult bool
	hasTagOrderErr    error

	orderTagsErr error
	orderCalls   []orderCall
}

func newMockTagOrderer() *mockTagOrderer {
	return &mockTagOrderer{mockTagCheckerTagger: *newMockTagCheckerTagger()}
}

func (m *mockTagOrderer) Apply(path, tag string) error {
	if tag == m.failApplyTag {
		return errors.New("apply failed")
	}
	return m.mockTagCheckerTagger.Apply(path, tag)
}

func (m *mockTagOrderer) HasTagOrder(_ string, _ []string) (bool, error) {
	m.hasTagOrderCalls++
	return m.hasTagOrderResult, m.hasTagOrderErr
}

func (m *mockTagOrderer) OrderTags(path string, tags []string) error {
	m.orderCalls = append(m.orderCalls, orderCall{path: path, tags: append([]string(nil), tags...)})
	return m.orderTagsErr
}

// setupCubaseTwoRuleTarget builds a Cubase-like target whose Green rule
// (Audio has files) is evaluated before its Blue rule (has Mixdown), matching
// the intended "last matching rule wins the Finder color" ordering.
func setupCubaseTwoRuleTarget(t *testing.T) config.ResolvedTarget {
	t.Helper()
	registry := engine.NewRegistry()

	greenRule, err := registry.CreateRule("subdirectory_has_files", []string{"Audio"}, "any", "Green")
	if err != nil {
		t.Fatal(err)
	}
	blueRule, err := registry.CreateRule("has_subdirectory", []string{"Mixdown"}, "any", "Blue")
	if err != nil {
		t.Fatal(err)
	}

	return config.ResolvedTarget{
		Name:  "Cubase",
		Rules: []engine.TagRule{greenRule, blueRule},
	}
}

// setupOrderableProjectDir creates a directory that matches both rules of
// setupCubaseTwoRuleTarget (Audio with a file, and a Mixdown subdirectory).
func setupOrderableProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Audio"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Audio", "take.wav"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// setupSingleMatchProjectDir creates a directory that matches only the Green
// rule of setupCubaseTwoRuleTarget (no Mixdown subdirectory).
func setupSingleMatchProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Audio"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Audio", "take.wav"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEvaluateRules_TagOrder_ReordersWhenTagsNotOrdered(t *testing.T) {
	dir := setupOrderableProjectDir(t)
	tagger := newMockTagOrderer()
	tagger.hasTagOrderResult = false

	s := &Scanner{Tagger: tagger, Logger: slog.New(slog.DiscardHandler)}
	results := s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if len(tagger.orderCalls) != 1 {
		t.Fatalf("expected 1 OrderTags call, got %d", len(tagger.orderCalls))
	}
	call := tagger.orderCalls[0]
	if call.path != dir {
		t.Errorf("expected OrderTags path %q, got %q", dir, call.path)
	}
	wantTags := []string{"Green", "Blue"}
	if len(call.tags) != len(wantTags) || call.tags[0] != wantTags[0] || call.tags[1] != wantTags[1] {
		t.Errorf("expected OrderTags tags %v, got %v", wantTags, call.tags)
	}
}

func TestEvaluateRules_TagOrder_SkipsReorderWhenAlreadyOrdered(t *testing.T) {
	dir := setupOrderableProjectDir(t)
	tagger := newMockTagOrderer()
	tagger.hasTagOrderResult = true

	s := &Scanner{Tagger: tagger, Logger: slog.New(slog.DiscardHandler)}
	s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if len(tagger.orderCalls) != 0 {
		t.Errorf("expected 0 OrderTags calls when already ordered, got %d", len(tagger.orderCalls))
	}
}

func TestEvaluateRules_TagOrder_SkipsWhenSingleRuleMatches(t *testing.T) {
	dir := setupSingleMatchProjectDir(t)
	tagger := newMockTagOrderer()

	s := &Scanner{Tagger: tagger, Logger: slog.New(slog.DiscardHandler)}
	s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if tagger.hasTagOrderCalls != 0 {
		t.Errorf("expected HasTagOrder not called for a single matching rule, got %d calls", tagger.hasTagOrderCalls)
	}
	if len(tagger.orderCalls) != 0 {
		t.Errorf("expected OrderTags not called for a single matching rule, got %d calls", len(tagger.orderCalls))
	}
}

func TestEvaluateRules_TagOrder_SkipsInDryRun(t *testing.T) {
	dir := setupOrderableProjectDir(t)
	tagger := newMockTagOrderer()

	s := &Scanner{Tagger: tagger, DryRun: true, Logger: slog.New(slog.DiscardHandler)}
	s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if tagger.hasTagOrderCalls != 0 {
		t.Errorf("expected HasTagOrder not called in dry run, got %d calls", tagger.hasTagOrderCalls)
	}
	if len(tagger.orderCalls) != 0 {
		t.Errorf("expected OrderTags not called in dry run, got %d calls", len(tagger.orderCalls))
	}
}

func TestEvaluateRules_TagOrder_SkipsInRemoveMode(t *testing.T) {
	dir := setupOrderableProjectDir(t)
	tagger := newMockTagOrderer()

	s := &Scanner{Tagger: tagger, RemoveMode: true, Logger: slog.New(slog.DiscardHandler)}
	s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if tagger.hasTagOrderCalls != 0 {
		t.Errorf("expected HasTagOrder not called in remove mode, got %d calls", tagger.hasTagOrderCalls)
	}
	if len(tagger.orderCalls) != 0 {
		t.Errorf("expected OrderTags not called in remove mode, got %d calls", len(tagger.orderCalls))
	}
}

func TestEvaluateRules_TagOrder_SkipsWhenFirstRuleApplyFails(t *testing.T) {
	dir := setupOrderableProjectDir(t)
	tagger := newMockTagOrderer()
	tagger.failApplyTag = "Green"

	s := &Scanner{Tagger: tagger, Logger: slog.New(slog.DiscardHandler)}
	results := s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Action != ActionSkipped {
		t.Errorf("expected first result action %s, got %s", ActionSkipped, results[0].Action)
	}
	if results[1].Action != ActionTagged {
		t.Errorf("expected second result action %s, got %s", ActionTagged, results[1].Action)
	}
	if tagger.hasTagOrderCalls != 0 {
		t.Errorf("expected HasTagOrder not called with only one eligible tag, got %d calls", tagger.hasTagOrderCalls)
	}
	if len(tagger.orderCalls) != 0 {
		t.Errorf("expected OrderTags not called with only one eligible tag, got %d calls", len(tagger.orderCalls))
	}
}

func TestEvaluateRules_TagOrder_LogsWarningWhenHasTagOrderFails(t *testing.T) {
	dir := setupOrderableProjectDir(t)
	tagger := newMockTagOrderer()
	tagger.hasTagOrderErr = errors.New("check boom")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := &Scanner{Tagger: tagger, Logger: logger}
	s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if len(tagger.orderCalls) != 0 {
		t.Errorf("expected OrderTags not called after HasTagOrder error, got %d calls", len(tagger.orderCalls))
	}
	if !strings.Contains(logBuf.String(), "failed to check tag order") {
		t.Errorf("expected %q in log output, got:\n%s", "failed to check tag order", logBuf.String())
	}
}

func TestEvaluateRules_TagOrder_LogsWarningWhenOrderTagsFails(t *testing.T) {
	dir := setupOrderableProjectDir(t)
	tagger := newMockTagOrderer()
	tagger.hasTagOrderResult = false
	tagger.orderTagsErr = errors.New("order boom")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := &Scanner{Tagger: tagger, Logger: logger}
	s.evaluateRules(dir, setupCubaseTwoRuleTarget(t))

	if !strings.Contains(logBuf.String(), "failed to order tags") {
		t.Errorf("expected %q in log output, got:\n%s", "failed to order tags", logBuf.String())
	}
}

// setupThreeRuleGreenGreenBlueTarget builds a target with two independently-evaluated rules
// that both apply Green, plus a third rule applying Blue, so tests can verify that the tag
// passed to OrderTags is de-duplicated by first occurrence even when the repeat comes from
// separate rule matches rather than a single caller-supplied slice.
func setupThreeRuleGreenGreenBlueTarget(t *testing.T) config.ResolvedTarget {
	t.Helper()
	registry := engine.NewRegistry()

	green1, err := registry.CreateRule("subdirectory_has_files", []string{"Audio"}, "any", "Green")
	if err != nil {
		t.Fatal(err)
	}
	green2, err := registry.CreateRule("has_subdirectory", []string{"Extra"}, "any", "Green")
	if err != nil {
		t.Fatal(err)
	}
	blue, err := registry.CreateRule("has_subdirectory", []string{"Mixdown"}, "any", "Blue")
	if err != nil {
		t.Fatal(err)
	}

	return config.ResolvedTarget{
		Name:  "Cubase",
		Rules: []engine.TagRule{green1, green2, blue},
	}
}

func TestEvaluateRules_TagOrder_DedupsRepeatedTagFromMultipleRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Audio"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Audio", "take.wav"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Extra"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}

	tagger := newMockTagOrderer()
	tagger.hasTagOrderResult = false

	s := &Scanner{Tagger: tagger, Logger: slog.New(slog.DiscardHandler)}
	results := s.evaluateRules(dir, setupThreeRuleGreenGreenBlueTarget(t))

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if len(tagger.orderCalls) != 1 {
		t.Fatalf("expected 1 OrderTags call, got %d", len(tagger.orderCalls))
	}
	wantTags := []string{"Green", "Blue"}
	call := tagger.orderCalls[0]
	if len(call.tags) != len(wantTags) || call.tags[0] != wantTags[0] || call.tags[1] != wantTags[1] {
		t.Errorf("expected OrderTags tags %v (first-occurrence de-dup across rules), got %v", wantTags, call.tags)
	}
}

func setupCubaseTarget(t *testing.T) config.ResolvedTarget {
	t.Helper()
	registry := engine.NewRegistry()

	ind, err := registry.CreateIndicator("file_extension", ".cpr")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := registry.CreateRule("has_subdirectory", []string{"Mixdown"}, "any", "Blue")
	if err != nil {
		t.Fatal(err)
	}

	return config.ResolvedTarget{
		Name:       "Cubase",
		Indicators: []engine.Indicator{ind},
		Rules:      []engine.TagRule{rule},
	}
}

func TestScan_FindsNestedProject(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Music", "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != projectDir {
		t.Errorf("expected path %s, got %s", projectDir, results[0].Path)
	}
	if results[0].Tag != "Blue" {
		t.Errorf("expected tag Blue, got %s", results[0].Tag)
	}
	if results[0].Action != ActionTagged {
		t.Errorf("expected action %s, got %s", ActionTagged, results[0].Action)
	}
	if len(tagger.applied) != 1 {
		t.Errorf("expected 1 apply call, got %d", len(tagger.applied))
	}
}

func TestScan_NoMatchWithoutMixdown(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track2")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track2.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for project without Mixdown, got %d", len(results))
	}
}

func TestScan_RemoveMode(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionUntagged {
		t.Errorf("expected action %s, got %s", ActionUntagged, results[0].Action)
	}
	if len(tagger.removed) != 1 {
		t.Errorf("expected 1 remove call, got %d", len(tagger.removed))
	}
}

func TestScan_SkipDirOptimization(t *testing.T) {
	root := t.TempDir()

	outerProject := filepath.Join(root, "Outer")
	innerProject := filepath.Join(outerProject, "Inner")
	if err := os.MkdirAll(filepath.Join(innerProject, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outerProject, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outerProject, "Outer.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(innerProject, "Inner.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (SkipDir optimization), got %d", len(results))
	}
	if results[0].Path != outerProject {
		t.Errorf("expected outer project, got %s", results[0].Path)
	}
}

func TestScan_UnreadableDirectory(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	restricted := filepath.Join(root, "aaa-restricted")
	if err := os.Mkdir(restricted, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(restricted, 0755) }()

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result despite unreadable dir, got %d", len(results))
	}
}

func TestScan_MultipleRoots(t *testing.T) {
	root1 := t.TempDir()
	root2 := t.TempDir()

	for _, root := range []string{root1, root2} {
		projectDir := filepath.Join(root, "Track")
		if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "Track.cpr"), []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root1, root2})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results from 2 roots, got %d", len(results))
	}
}

func TestScan_DebugLogging(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		Logger:  logger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	output := buf.String()
	for _, expected := range []string{"visiting directory", "target matched", "skipping subtree", "tag applied"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected %q in debug output, got:\n%s", expected, output)
		}
	}
}

func TestScan_OnVisitCallback(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	projectDir2 := filepath.Join(root, "Track2")
	if err := os.MkdirAll(projectDir2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir2, "Track2.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "Other"), 0755); err != nil {
		t.Fatal(err)
	}

	var events []ScanEvent
	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		OnVisit: func(e ScanEvent) { events = append(events, e) },
	}

	_, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	var enter, match, skip int
	for _, e := range events {
		switch e.Kind {
		case EventEnter:
			enter++
		case EventMatch:
			match++
			if e.TargetName != "Cubase" {
				t.Errorf("expected target Cubase, got %s", e.TargetName)
			}
			if e.Tag != "Blue" {
				t.Errorf("expected tag Blue, got %s", e.Tag)
			}
		case EventSkip:
			skip++
			if e.TargetName != "Cubase" {
				t.Errorf("expected skip target Cubase, got %s", e.TargetName)
			}
		}
	}

	if enter == 0 {
		t.Error("expected at least one EventEnter")
	}
	if match != 1 {
		t.Errorf("expected 1 EventMatch, got %d", match)
	}
	if skip != 1 {
		t.Errorf("expected 1 EventSkip, got %d", skip)
	}
}

func TestScan_OnVisitEmitsOnlyDirectories(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "audio.wav"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	restricted := filepath.Join(root, "aaa-restricted")
	if err := os.Mkdir(restricted, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(restricted, 0755) }()

	var events []ScanEvent
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  &mockTagger{},
		OnVisit: func(e ScanEvent) { events = append(events, e) },
	}

	_, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	for _, e := range events {
		info, err := os.Lstat(e.Path)
		if err != nil {
			if e.Kind == EventWarn {
				continue
			}
			t.Errorf("event path %q does not exist (kind=%d)", e.Path, e.Kind)
			continue
		}
		if !info.IsDir() {
			t.Errorf("event emitted for non-directory path: %s (kind=%d)", e.Path, e.Kind)
		}
	}
}

func TestScan_IndicatorError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	target := config.ResolvedTarget{
		Name:       "Broken",
		Indicators: []engine.Indicator{&errorIndicator{err: errors.New("indicator boom")}},
		Rules:      []engine.TagRule{},
	}

	var events []ScanEvent
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := &Scanner{
		Targets: []config.ResolvedTarget{target},
		Tagger:  &mockTagger{},
		Logger:  logger,
		OnVisit: func(e ScanEvent) { events = append(events, e) },
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	if !strings.Contains(buf.String(), "indicator evaluation failed") {
		t.Errorf("expected indicator warning in log, got:\n%s", buf.String())
	}

	var warns int
	for _, e := range events {
		if e.Kind == EventWarn {
			warns++
		}
	}
	if warns == 0 {
		t.Error("expected at least one EventWarn for indicator error")
	}
}

func TestScan_RuleEvaluationError(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	registry := engine.NewRegistry()
	ind, err := registry.CreateIndicator("file_extension", ".cpr")
	if err != nil {
		t.Fatal(err)
	}

	target := config.ResolvedTarget{
		Name:       "Cubase",
		Indicators: []engine.Indicator{ind},
		Rules:      []engine.TagRule{&errorRule{err: errors.New("rule boom")}},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := &Scanner{
		Targets: []config.ResolvedTarget{target},
		Tagger:  &mockTagger{},
		Logger:  logger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	if !strings.Contains(buf.String(), "rule evaluation failed") {
		t.Errorf("expected rule warning in log, got:\n%s", buf.String())
	}
}

func TestScan_RuleErrorEmitsWarnEvent(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	registry := engine.NewRegistry()
	ind, err := registry.CreateIndicator("file_extension", ".cpr")
	if err != nil {
		t.Fatal(err)
	}

	target := config.ResolvedTarget{
		Name:       "Cubase",
		Indicators: []engine.Indicator{ind},
		Rules:      []engine.TagRule{&errorRule{err: errors.New("rule boom")}},
	}

	var events []ScanEvent

	s := &Scanner{
		Targets: []config.ResolvedTarget{target},
		Tagger:  &mockTagger{},
		OnVisit: func(e ScanEvent) { events = append(events, e) },
	}

	results, scanErr := s.Scan([]string{root})
	if scanErr != nil {
		t.Fatal(scanErr)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	var warnEvents []ScanEvent
	for _, e := range events {
		if e.Kind == EventWarn {
			warnEvents = append(warnEvents, e)
		}
	}

	if len(warnEvents) != 1 {
		t.Fatalf("expected exactly 1 EventWarn, got %d: %+v", len(warnEvents), warnEvents)
	}

	warn := warnEvents[0]
	if warn.Path != projectDir {
		t.Errorf("expected Path %q, got %q", projectDir, warn.Path)
	}
	if warn.Message != "rule boom" {
		t.Errorf("expected Message %q, got %q", "rule boom", warn.Message)
	}
	if warn.TargetName != "" {
		t.Errorf("expected empty TargetName, got %q", warn.TargetName)
	}
}

func TestScan_ApplyTaggerError(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  &failTagger{applyErr: errors.New("xattr fail")},
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionSkipped {
		t.Errorf("expected action %s, got %s", ActionSkipped, results[0].Action)
	}
}

func TestScan_RemoveModeTaggerError(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     &failTagger{removeErr: errors.New("xattr fail")},
		RemoveMode: true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionSkipped {
		t.Errorf("expected action %s, got %s", ActionSkipped, results[0].Action)
	}
}

func TestScan_DryRun(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		DryRun:  true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldTag {
		t.Errorf("expected action %s, got %s", ActionWouldTag, results[0].Action)
	}
	if len(tagger.applied) != 0 {
		t.Errorf("expected 0 apply calls in dry run, got %d", len(tagger.applied))
	}
}

func TestScan_DryRunRemoveMode(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTagger{}
	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
		DryRun:     true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldUntag {
		t.Errorf("expected action %s, got %s", ActionWouldUntag, results[0].Action)
	}
	if len(tagger.removed) != 0 {
		t.Errorf("expected 0 remove calls in dry run, got %d", len(tagger.removed))
	}
}

func TestScan_DryRunWithFailingTagger(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &failTagger{applyErr: errors.New("xattr fail"), removeErr: errors.New("xattr fail")}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		DryRun:  true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldTag {
		t.Errorf("expected action %s despite failing tagger, got %s", ActionWouldTag, results[0].Action)
	}
}

func TestScan_NonexistentRoot(t *testing.T) {
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  &mockTagger{},
	}

	_, err := s.Scan([]string{"/nonexistent/root/path"})
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
	if !strings.Contains(err.Error(), "scanning") {
		t.Errorf("expected 'scanning' in error message, got: %v", err)
	}
}

func setupProjectDir(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	return root, projectDir
}

func TestScan_AlreadyTagged(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockTagCheckerTagger()
	tagger.setTag(projectDir, "Blue")

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionAlreadyTagged {
		t.Errorf("expected action %s, got %s", ActionAlreadyTagged, results[0].Action)
	}
	if len(tagger.applied) != 0 {
		t.Errorf("expected 0 apply calls, got %d", len(tagger.applied))
	}
}

func TestScan_AlreadyTagged_WithoutChecker(t *testing.T) {
	root, _ := setupProjectDir(t)

	tagger := &mockTagger{}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionTagged {
		t.Errorf("expected action %s, got %s", ActionTagged, results[0].Action)
	}
	if len(tagger.applied) != 1 {
		t.Errorf("expected 1 apply call, got %d", len(tagger.applied))
	}
}

func TestScan_AlreadyTagged_RemoveModeSkipsCheck(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "Track1")
	if err := os.MkdirAll(filepath.Join(projectDir, "Mixdown"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Track1.cpr"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	tagger := &mockTaggerWithChecker{
		hasTag: map[string]bool{projectDir + "\x00" + "Blue": true},
	}
	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// In remove mode, HasTag should NOT be checked — action should be "untagged", not "already_tagged"
	if results[0].Action != ActionUntagged {
		t.Errorf("expected action untagged in remove mode, got %s", results[0].Action)
	}
}

func TestScan_AlreadyTagged_HasTagError(t *testing.T) {
	root, _ := setupProjectDir(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tagger := &failingTagCheckerTagger{hasTagErr: errors.New("xattr read fail")}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		Logger:  logger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should fall through to Apply after HasTag error
	if results[0].Action != ActionTagged {
		t.Errorf("expected action %s (fallthrough after HasTag error), got %s", ActionTagged, results[0].Action)
	}
	// Warning should be logged about the HasTag failure
	if !strings.Contains(logBuf.String(), "HasTag check failed") {
		t.Errorf("expected warning about failed tag check in logs, got: %s", logBuf.String())
	}
}

func TestScan_DryRunAlreadyTagged(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockTagCheckerTagger()
	tagger.setTag(projectDir, "Blue")

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		DryRun:  true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionAlreadyTagged {
		t.Errorf("expected action %s, got %s", ActionAlreadyTagged, results[0].Action)
	}
	if len(tagger.applied) != 0 {
		t.Errorf("expected 0 apply calls in dry run, got %d", len(tagger.applied))
	}
}

func TestScan_DryRunNotYetTagged(t *testing.T) {
	root, _ := setupProjectDir(t)

	tagger := newMockTagCheckerTagger()
	// No tags set — directory is not yet tagged

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		DryRun:  true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldTag {
		t.Errorf("expected action %s, got %s", ActionWouldTag, results[0].Action)
	}
}

func TestScan_DryRunRemoveModeTagPresent(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockTagCheckerTagger()
	tagger.setTag(projectDir, "Blue")

	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
		DryRun:     true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldUntag {
		t.Errorf("expected action %s, got %s", ActionWouldUntag, results[0].Action)
	}
	if len(tagger.removed) != 0 {
		t.Errorf("expected 0 remove calls in dry run, got %d", len(tagger.removed))
	}
}

func TestScan_DryRunRemoveModeTagNotPresent(t *testing.T) {
	root, _ := setupProjectDir(t)

	tagger := newMockTagCheckerTagger()
	// No tags set — nothing to remove

	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
		DryRun:     true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionSkipped {
		t.Errorf("expected action %s, got %s", ActionSkipped, results[0].Action)
	}
	if len(tagger.removed) != 0 {
		t.Errorf("expected 0 remove calls in dry run, got %d", len(tagger.removed))
	}
}

func TestScan_DryRunHasTagError(t *testing.T) {
	root, _ := setupProjectDir(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tagger := &failingTagCheckerTagger{hasTagErr: errors.New("xattr read fail")}
	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		DryRun:  true,
		Logger:  logger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should still produce a result (would_tag) despite HasTag error
	if results[0].Action != ActionWouldTag {
		t.Errorf("expected action %s, got %s", ActionWouldTag, results[0].Action)
	}
	// Warning should be logged about the HasTag failure
	if !strings.Contains(logBuf.String(), "failed to check tag") {
		t.Errorf("expected warning about failed tag check in logs, got: %s", logBuf.String())
	}
}

func TestScan_AddMode_HasTagTrue_NeedsRepairTrue_AppliesAndTags(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockRepairCheckerTagger()
	tagger.setTag(projectDir, "Blue")
	tagger.needsRepair[projectDir] = true

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionTagged {
		t.Errorf("expected action %s when the tag needs repair, got %s", ActionTagged, results[0].Action)
	}
	if len(tagger.applied) != 1 {
		t.Errorf("expected 1 apply call when the tag needs repair, got %d", len(tagger.applied))
	}
}

func TestScan_AddMode_HasTagTrue_NeedsRepairFalse_AlreadyTagged_ApplyNotCalled(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockRepairCheckerTagger()
	tagger.setTag(projectDir, "Blue")
	// needsRepair left false for projectDir.

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionAlreadyTagged {
		t.Errorf("expected action %s, got %s", ActionAlreadyTagged, results[0].Action)
	}
	if len(tagger.applied) != 0 {
		t.Errorf("expected 0 apply calls when the tag does not need repair, got %d", len(tagger.applied))
	}
}

func TestScan_DryRun_AddMode_HasTagTrue_NeedsRepairTrue_WouldTag(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockRepairCheckerTagger()
	tagger.setTag(projectDir, "Blue")
	tagger.needsRepair[projectDir] = true

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		DryRun:  true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldTag {
		t.Errorf("expected action %s when the tag needs repair (dry run), got %s", ActionWouldTag, results[0].Action)
	}
	if len(tagger.applied) != 0 {
		t.Errorf("expected 0 apply calls in dry run, got %d", len(tagger.applied))
	}
}

func TestScan_AddMode_NeedsRepairError_FallsThroughToApply(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tagger := newMockRepairCheckerTagger()
	tagger.setTag(projectDir, "Blue")
	tagger.needsRepairErr = errors.New("repair check boom")

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		Logger:  logger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionTagged {
		t.Errorf("expected action %s (fallthrough after NeedsRepair error), got %s", ActionTagged, results[0].Action)
	}
	if len(tagger.applied) != 1 {
		t.Errorf("expected 1 apply call after NeedsRepair error, got %d", len(tagger.applied))
	}
	if !strings.Contains(logBuf.String(), "repair check failed, falling through to Apply") {
		t.Errorf("expected %q in log output, got:\n%s", "repair check failed, falling through to Apply", logBuf.String())
	}
}

func TestScan_DryRun_AddMode_NeedsRepairError_LogsAndWouldTag(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tagger := newMockRepairCheckerTagger()
	tagger.setTag(projectDir, "Blue")
	tagger.needsRepairErr = errors.New("repair check boom")

	s := &Scanner{
		Targets: []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:  tagger,
		DryRun:  true,
		Logger:  logger,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldTag {
		t.Errorf("expected action %s despite NeedsRepair error (dry run), got %s", ActionWouldTag, results[0].Action)
	}
	if !strings.Contains(logBuf.String(), "failed to check tag repair (dry run)") {
		t.Errorf("expected %q in log output, got:\n%s", "failed to check tag repair (dry run)", logBuf.String())
	}
}

// TestScan_DryRunRemoveMode_HasTagTrue_NeedsRepairTrue_WouldUntag_Unaffected pins that remove
// mode reports presence only: a Tagger that also implements TagRepairChecker must not change
// remove-mode's dry-run outcome, since a removal doesn't care whether the stored tag is
// canonical.
func TestScan_DryRunRemoveMode_HasTagTrue_NeedsRepairTrue_WouldUntag_Unaffected(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockRepairCheckerTagger()
	tagger.setTag(projectDir, "Blue")
	tagger.needsRepair[projectDir] = true

	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
		DryRun:     true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionWouldUntag {
		t.Errorf("expected action %s regardless of NeedsRepair in remove mode, got %s", ActionWouldUntag, results[0].Action)
	}
}

// TestScan_RemoveMode_HasTagTrue_NeedsRepairTrue_Untagged_Unaffected is the real (non-dry-run)
// counterpart: remove mode never consults TagRepairChecker.
func TestScan_RemoveMode_HasTagTrue_NeedsRepairTrue_Untagged_Unaffected(t *testing.T) {
	root, projectDir := setupProjectDir(t)

	tagger := newMockRepairCheckerTagger()
	tagger.setTag(projectDir, "Blue")
	tagger.needsRepair[projectDir] = true

	s := &Scanner{
		Targets:    []config.ResolvedTarget{setupCubaseTarget(t)},
		Tagger:     tagger,
		RemoveMode: true,
	}

	results, err := s.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionUntagged {
		t.Errorf("expected action %s regardless of NeedsRepair in remove mode, got %s", ActionUntagged, results[0].Action)
	}
	if len(tagger.removed) != 1 {
		t.Errorf("expected 1 remove call, got %d", len(tagger.removed))
	}
	if tagger.needsRepairCalls != 0 {
		t.Errorf("expected NeedsRepair not to be called in remove mode, got %d calls", tagger.needsRepairCalls)
	}
}
