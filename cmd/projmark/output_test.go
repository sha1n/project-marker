package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/sha1n/project-marker/internal/scanner"
)

func TestVerboseHandler_EventKinds(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, false)

	handler(scanner.ScanEvent{Kind: scanner.EventEnter, Path: "/root/Music/Track1"})
	handler(scanner.ScanEvent{Kind: scanner.EventMatch, Path: "/root/Music/Track1", TargetName: "Cubase", Tag: "Blue", Action: "tagged"})
	handler(scanner.ScanEvent{Kind: scanner.EventSkip, Path: "/root/Music/Track2", TargetName: "Cubase"})
	handler(scanner.ScanEvent{Kind: scanner.EventWarn, Path: "/root/Music/Broken", Message: "permission denied"})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), output)
	}

	if !strings.Contains(lines[0], "◦") || !strings.Contains(lines[0], "Track1") {
		t.Errorf("EventEnter line unexpected: %s", lines[0])
	}
	if strings.Contains(lines[0], "Music/Track1") {
		t.Errorf("EventEnter should show directory name only, not full path: %s", lines[0])
	}

	if !strings.Contains(lines[1], "●") || !strings.Contains(lines[1], "Cubase [Blue]") {
		t.Errorf("EventMatch line unexpected: %s", lines[1])
	}

	if !strings.Contains(lines[2], "◦") || !strings.Contains(lines[2], "no matching rule") {
		t.Errorf("EventSkip line unexpected: %s", lines[2])
	}

	if !strings.Contains(lines[3], "⚠") || !strings.Contains(lines[3], "permission denied") {
		t.Errorf("EventWarn line unexpected: %s", lines[3])
	}
}

func TestVerboseHandler_TreeIndentation(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, false)

	handler(scanner.ScanEvent{Kind: scanner.EventEnter, Path: "/root/Music"})
	handler(scanner.ScanEvent{Kind: scanner.EventEnter, Path: "/root/Music/Track1"})
	handler(scanner.ScanEvent{Kind: scanner.EventEnter, Path: "/root/Music/Track1/Audio"})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), buf.String())
	}

	indent0 := len(lines[0]) - len(strings.TrimLeft(lines[0], " "))
	indent1 := len(lines[1]) - len(strings.TrimLeft(lines[1], " "))
	indent2 := len(lines[2]) - len(strings.TrimLeft(lines[2], " "))

	if indent1 <= indent0 {
		t.Errorf("depth 1 indent (%d) should be greater than depth 0 (%d)", indent1, indent0)
	}
	if indent2 <= indent1 {
		t.Errorf("depth 2 indent (%d) should be greater than depth 1 (%d)", indent2, indent1)
	}

	if !strings.Contains(lines[0], "Music") {
		t.Errorf("line 0 should contain 'Music': %s", lines[0])
	}
	if strings.Contains(lines[1], "Music/") {
		t.Errorf("line 1 should show 'Track1' only, not full path: %s", lines[1])
	}
	if strings.Contains(lines[2], "Track1/") {
		t.Errorf("line 2 should show 'Audio' only, not full path: %s", lines[2])
	}
}

func TestVerboseHandler_SkipsRoot(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, false)

	handler(scanner.ScanEvent{Kind: scanner.EventEnter, Path: "/root"})

	if buf.Len() > 0 {
		t.Errorf("should not output anything for root directory, got: %s", buf.String())
	}
}

func TestVerboseHandler_NoANSIWithoutColor(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, false)

	handler(scanner.ScanEvent{Kind: scanner.EventMatch, Path: "/root/Track", TargetName: "Cubase", Tag: "Blue", Action: "tagged"})

	if strings.Contains(buf.String(), "\033[") {
		t.Error("expected no ANSI escape codes with color=false")
	}
}

func TestVerboseHandler_ANSIWithColor(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, true)

	handler(scanner.ScanEvent{Kind: scanner.EventMatch, Path: "/root/Track", TargetName: "Cubase", Tag: "Blue", Action: "tagged"})

	if !strings.Contains(buf.String(), "\033[") {
		t.Error("expected ANSI escape codes with color=true")
	}
}

func TestVerboseHandler_AllEventKindsWithColor(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, true)

	handler(scanner.ScanEvent{Kind: scanner.EventEnter, Path: "/root/Track1"})
	handler(scanner.ScanEvent{Kind: scanner.EventSkip, Path: "/root/Track2", TargetName: "Cubase"})
	handler(scanner.ScanEvent{Kind: scanner.EventWarn, Path: "/root/Broken", Message: "access denied"})

	output := buf.String()
	if !strings.Contains(output, colorDim) {
		t.Error("expected dim color for EventEnter")
	}
	if !strings.Contains(output, colorYellow) {
		t.Error("expected yellow color for EventWarn")
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		if !strings.Contains(line, colorReset) {
			t.Errorf("line %d missing color reset: %s", i, line)
		}
	}
}

func TestVerboseHandler_UntaggedColor(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, true)

	handler(scanner.ScanEvent{Kind: scanner.EventMatch, Path: "/root/Track", TargetName: "Cubase", Tag: "Blue", Action: "untagged"})

	if !strings.Contains(buf.String(), "\033[36m") {
		t.Error("expected cyan ANSI code for untagged action")
	}
}

func TestVerboseHandler_UnknownEventKind(t *testing.T) {
	var buf bytes.Buffer
	handler := verboseHandler([]string{"/root"}, &buf, false)

	handler(scanner.ScanEvent{Kind: scanner.EventKind(99), Path: "/root/Track"})

	if buf.Len() > 0 {
		t.Errorf("expected no output for unknown event kind, got: %s", buf.String())
	}
}

func TestIsTTY_ClosedFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tty-test")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if isTTY(f) {
		t.Error("expected false for closed file")
	}
}
