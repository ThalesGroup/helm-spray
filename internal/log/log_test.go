package log

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func captureStderr(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestInfo_Level1_NoParams(t *testing.T) {
	out := captureStdout(func() { Info(1, "hello") })
	if !strings.Contains(out, "[spray] hello") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestInfo_Level1_WithParams(t *testing.T) {
	out := captureStdout(func() { Info(1, "val %d", 42) })
	if !strings.Contains(out, "val 42") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestInfo_Level2(t *testing.T) {
	out := captureStdout(func() { Info(2, "msg") })
	if !strings.Contains(out, "  > ") {
		t.Errorf("expected level-2 indent, got %q", out)
	}
}

func TestInfo_Level3(t *testing.T) {
	out := captureStdout(func() { Info(3, "msg") })
	if !strings.Contains(out, "    o ") {
		t.Errorf("expected level-3 indent, got %q", out)
	}
}

func TestInfo_Level4(t *testing.T) {
	out := captureStdout(func() { Info(4, "msg") })
	if !strings.Contains(out, "      - ") {
		t.Errorf("expected level-4 indent, got %q", out)
	}
}

func TestInfo_Level5(t *testing.T) {
	out := captureStdout(func() { Info(5, "msg") })
	if !strings.Contains(out, "        . ") {
		t.Errorf("expected level-5 indent, got %q", out)
	}
}

func TestInfo_Level0_NoExtraIndent(t *testing.T) {
	out := captureStdout(func() { Info(0, "bare") })
	if !strings.Contains(out, "[spray] bare") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestError_NoParams(t *testing.T) {
	out := captureStderr(func() { Error("something went wrong") })
	if !strings.Contains(out, "something went wrong") {
		t.Errorf("unexpected stderr: %q", out)
	}
}

func TestError_WithParams(t *testing.T) {
	out := captureStderr(func() { Error("code %d", 500) })
	if !strings.Contains(out, "code 500") {
		t.Errorf("unexpected stderr: %q", out)
	}
}

func TestWithNumberedLines_MultiLine(t *testing.T) {
	out := captureStdout(func() {
		WithNumberedLines(1, "first\nsecond\nthird\n")
	})
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") || !strings.Contains(out, "third") {
		t.Errorf("expected all lines in output, got %q", out)
	}
}

func TestWithNumberedLines_SingleLineNoTrailingNewline(t *testing.T) {
	out := captureStdout(func() {
		WithNumberedLines(1, "only line")
	})
	if !strings.Contains(out, "only line") {
		t.Errorf("expected line in output, got %q", out)
	}
}

func TestWithNumberedLines_EmptyString(t *testing.T) {
	// Empty string: numberOfLines=0, loop in digit-count never runs (numberOfDigits=0 means format "[%0d] %s")
	// Scanner yields no lines, so nothing is printed
	out := captureStdout(func() {
		WithNumberedLines(1, "")
	})
	_ = out // no assertion: just verifying it doesn't panic
}

func TestWithNumberedLines_ManyLines_NumberingFormat(t *testing.T) {
	// Build 11 lines so numberOfLines=11 → numberOfDigits=2
	lines := make([]string, 11)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i)
	}
	content := strings.Join(lines, "\n") + "\n"
	out := captureStdout(func() {
		WithNumberedLines(1, content)
	})
	if !strings.Contains(out, "line0") || !strings.Contains(out, "line10") {
		t.Errorf("expected all 11 lines in output, got %q", out)
	}
}
