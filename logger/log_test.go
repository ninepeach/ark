package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// --- Helpers ---

// newTestStdLogger returns a logger writing to in-memory buffer.
func newTestStdLogger(t *testing.T) (*Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	l := NewStdLogger(true, true, true, false, false)
	l.logger.SetOutput(&buf)
	return l, &buf
}

// newTestFileLogger creates a temporary file logger.
func newTestFileLogger(t *testing.T) (*Logger, string) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "test.log")
	l, err := NewFileLogger(tmp, true, true, true, false)
	if err != nil {
		t.Fatalf("failed to create NewFileLogger: %v", err)
	}
	return l, tmp
}

// assertContains checks whether substring exists in a buffer.
func assertContains(t *testing.T, buf *bytes.Buffer, want string) {
	t.Helper()
	got := buf.String()
	if !bytes.Contains([]byte(got), []byte(want)) {
		t.Fatalf("expected output to contain %q, got: %q", want, got)
	}
}

// --- Tests ---

// Test standard logger basic output
func TestStdLoggerBasic(t *testing.T) {
	l, buf := newTestStdLogger(t)

	l.Noticef("hello %s", "world")
	assertContains(t, buf, "[INF] hello world")

	buf.Reset()
	l.Warnf("warn %d", 99)
	assertContains(t, buf, "[WRN] warn 99")

	buf.Reset()
	l.Errorf("error %d", 7)
	assertContains(t, buf, "[ERR] error 7")

	buf.Reset()
	l.Debugf("debugging")
	assertContains(t, buf, "[DBG] debugging")

	buf.Reset()
	l.Tracef("trace here")
	assertContains(t, buf, "[TRC] trace here")
}

// Test LogUTC flag toggles logger flags correctly
func TestLoggerUTC(t *testing.T) {
	l := NewStdLogger(true, true, true, false, false, LogUTC(true))
	var buf bytes.Buffer
	l.logger.SetOutput(&buf)

	l.Noticef("utc log")
	assertContains(t, &buf, "[INF] utc log")
}

// Test file logger writes to file
func TestFileLoggerWrite(t *testing.T) {
	l, fname := newTestFileLogger(t)

	l.Noticef("file written")

	data, err := os.ReadFile(fname)
	if err != nil {
		t.Fatalf("cannot read log file: %v", err)
	}
	if !bytes.Contains(data, []byte("[INF] file written")) {
		t.Fatalf("log file does not contain expected data")
	}
}

// Test rotation triggers (very small limit)
func TestFileRotation(t *testing.T) {
	l, fname := newTestFileLogger(t)

	if err := l.SetSizeLimit(50); err != nil {
		t.Fatalf("SetSizeLimit error: %v", err)
	}

	// write enough logs to trigger rotation
	for i := 0; i < 20; i++ {
		l.Noticef("hello %d", i)
	}

	// expect at least:
	// fname
	// fname.YYYY.MM...
	dir := filepath.Dir(fname)
	files, _ := os.ReadDir(dir)

	found := false
	for _, f := range files {
		if f.Name() != filepath.Base(fname) && 
			len(f.Name()) > len(filepath.Base(fname)) {
			found = true
		}
	}

	if !found {
		t.Fatalf("expected rotated backup file, but none found")
	}
}

// Close should close underlying file
func TestFileLoggerClose(t *testing.T) {
	l, fname := newTestFileLogger(t)

	if err := l.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// File still exists
	if _, err := os.Stat(fname); err != nil {
		t.Fatalf("expected log file to exist after Close(), got error: %v", err)
	}
}