package output

import (
	"io"
	"log/slog"
)

// LevelSilent is above every level slog defines, so a logger set to it emits
// nothing at all — not even slog.Error.
//
// It is the default because slog is a diagnostic channel for someone debugging
// specs, not a reporting channel for someone using it: everything a user should
// see goes through Writer. Making silence the level rather than a convention is
// what keeps that true when the first slog.Info is added to the tree.
const LevelSilent = slog.LevelError + 1

// SetupLogger installs the process-wide default slog logger on w and returns the
// LevelVar gating it. debug selects slog.LevelDebug; otherwise the level is
// LevelSilent and nothing is written. The handler matches format, so
// `--debug -o json` yields a stderr stream that is JSON all the way down.
//
// The level is returned rather than baked in because cobra parses persistent
// flags only after the command tree is built: NewApp calls this with a silent
// default so a failure before flag parsing still has somewhere to go, and
// PersistentPreRunE calls it again with the command's own stderr and the
// resolved flags.
func SetupLogger(w io.Writer, format Format, debug bool) *slog.LevelVar {
	level := new(slog.LevelVar)
	if debug {
		level.Set(slog.LevelDebug)
	} else {
		level.Set(LevelSilent)
	}

	slog.SetDefault(slog.New(newHandler(w, format, level)))

	return level
}

// newHandler builds the handler for a format: JSON records for FormatJSON, text
// for everything else. An unrecognised format still has to log somewhere, and
// text is what a human reading a terminal wants.
func newHandler(w io.Writer, format Format, level slog.Leveler) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}

	if format == FormatJSON {
		return slog.NewJSONHandler(w, opts)
	}

	return slog.NewTextHandler(w, opts)
}
