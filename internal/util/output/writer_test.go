package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/util/output"
)

// esc is the prefix of every ANSI style sequence lipgloss emits.
const esc = "\x1b["

func TestPrettyWriter_ColourFollowsEnviron(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		want    string
	}{
		{
			name:    "empty environ strips every sequence",
			environ: []string{},
			want:    "info hello\n",
		},
		{
			name:    "CLICOLOR_FORCE renders colour to a non-terminal",
			environ: []string{"CLICOLOR_FORCE=1", "TERM=xterm", "COLORTERM=truecolor"},
			want:    "\x1b[1;38;5;12minfo\x1b[m hello\n",
		},
		{
			// NO_COLOR disables colour but keeps bold, per no-color.org.
			name:    "NO_COLOR wins over CLICOLOR_FORCE",
			environ: []string{"NO_COLOR=1", "CLICOLOR_FORCE=1", "TERM=xterm", "COLORTERM=truecolor"},
			want:    "\x1b[1minfo\x1b[m hello\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			output.NewPrettyWriter(&buf, &bytes.Buffer{}, tt.environ).Info("hello")

			if got := buf.String(); got != tt.want {
				t.Errorf("Info rendered %q, want %q", got, tt.want)
			}
		})
	}
}

// The environment is read at construction from the slice the caller supplies, so
// the same call renders the same bytes under a terminal and in CI.
func TestPrettyWriter_IgnoresProcessEnviron(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")

	var buf bytes.Buffer

	output.NewPrettyWriter(&buf, &bytes.Buffer{}, []string{}).Info("hello")

	if strings.Contains(buf.String(), esc) {
		t.Errorf("process environment leaked into the rendering: %q", buf.String())
	}
}

// Each stream gets its own colorprofile.Writer, so a plain stdout does not force
// stderr to be plain too.
func TestPrettyWriter_StreamsAreWrappedIndependently(t *testing.T) {
	var out, errBuf bytes.Buffer

	w := output.NewPrettyWriter(&out, &errBuf, []string{"CLICOLOR_FORCE=1", "TERM=xterm", "COLORTERM=truecolor"})
	w.Info("info")
	w.Warn("warn")

	if !strings.Contains(out.String(), esc) {
		t.Errorf("stdout not styled: %q", out.String())
	}

	if !strings.Contains(errBuf.String(), esc) {
		t.Errorf("stderr not styled: %q", errBuf.String())
	}
}
