package output_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/output"
)

func TestInfo_NonEmpty(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output.Info("hello %s", "world")

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() == 0 {
		t.Error("Info() produced no output")
	}
}

func TestWarn_NonEmpty(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	output.Warn("something wrong")

	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() == 0 {
		t.Error("Warn() produced no output")
	}
}

func TestError_NonEmpty(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	output.Error("fatal error")

	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() == 0 {
		t.Error("Error() produced no output")
	}
}

func TestDebug_DisabledByDefault(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output.SetDebug(false)
	output.Debug("should not appear")

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() != 0 {
		t.Error("Debug() produced output when debug is disabled")
	}
}

func TestDebug_EnabledProducesOutput(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output.SetDebug(true)
	output.Debug("visible debug message")
	output.SetDebug(false) // restore

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() == 0 {
		t.Error("Debug() produced no output when debug is enabled")
	}
}
