package output_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/util/output"
)

// restoreLogger puts the process-wide default logger back after a test has
// replaced it, so one case cannot leak its handler into the next.
func restoreLogger(t *testing.T) {
	t.Helper()

	previous := slog.Default()

	t.Cleanup(func() { slog.SetDefault(previous) })
}

// Silence is the level, not a convention: every level slog defines is below
// LevelSilent, so the first slog.Info someone adds to the tree still writes
// nothing on a run without --debug.
func TestSetupLogger_SilentWithoutDebug(t *testing.T) {
	restoreLogger(t)

	var buf bytes.Buffer

	output.SetupLogger(&buf, output.FormatPretty, false)

	slog.Debug("debug record")
	slog.Info("info record")
	slog.Warn("warn record")
	slog.Error("error record")

	if buf.Len() != 0 {
		t.Errorf("logged %q without --debug, want nothing", buf.String())
	}
}

func TestSetupLogger_DebugWritesToTheGivenStream(t *testing.T) {
	restoreLogger(t)

	var buf bytes.Buffer

	output.SetupLogger(&buf, output.FormatPretty, true)

	slog.Debug("debug record", "key", "value")

	got := buf.String()
	if !strings.Contains(got, "debug record") || !strings.Contains(got, "key=value") {
		t.Errorf("logged %q, want a text record carrying the message and its attribute", got)
	}
}

func TestSetupLogger_DebugJSONEmitsJSONRecords(t *testing.T) {
	restoreLogger(t)

	var buf bytes.Buffer

	output.SetupLogger(&buf, output.FormatJSON, true)

	slog.Debug("debug record", "key", "value")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal %q: %v", buf.String(), err)
	}

	if got, want := record["msg"], "debug record"; got != want {
		t.Errorf("msg = %v, want %v", got, want)
	}

	if got, want := record["key"], "value"; got != want {
		t.Errorf("key = %v, want %v", got, want)
	}
}

// The returned LevelVar is the handle on the installed logger, so a caller that
// resolves --debug later can raise the level without rebuilding the handler.
func TestSetupLogger_ReturnsTheGatingLevel(t *testing.T) {
	restoreLogger(t)

	var buf bytes.Buffer

	level := output.SetupLogger(&buf, output.FormatPretty, false)
	if got := level.Level(); got != output.LevelSilent {
		t.Errorf("level = %v, want %v", got, output.LevelSilent)
	}

	level.Set(slog.LevelDebug)
	slog.Debug("now visible")

	if !strings.Contains(buf.String(), "now visible") {
		t.Errorf("logged %q, want the record after raising the returned level", buf.String())
	}
}
