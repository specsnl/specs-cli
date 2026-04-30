package template_test

import (
	"io"
	"log/slog"
)

// discardLogger returns a logger that silently discards all output.
// Used in tests where logging behaviour is not under test.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
