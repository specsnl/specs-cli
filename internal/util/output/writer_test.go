package output_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/output"
)

// esc is the prefix of every ANSI style sequence lipgloss emits.
const esc = "\x1b["

// stream names the buffer a writer method is expected to write to. Every case
// asserts the other stream stayed empty, which is the assertion the old
// buf.Len() != 0 tests could not make.
type stream int

const (
	streamStdout stream = iota
	streamStderr
)

func (s stream) String() string {
	if s == streamStdout {
		return "stdout"
	}

	return "stderr"
}

// capture runs call against a fresh writer and returns what landed on want,
// failing if anything landed on the other stream.
func capture(t *testing.T, out, errOut *bytes.Buffer, want stream) string {
	t.Helper()

	got, other, otherName := out, errOut, streamStderr
	if want == streamStderr {
		got, other, otherName = errOut, out, streamStdout
	}

	if other.Len() != 0 {
		t.Errorf("wrote %q to %s, expected output on %s only", other.String(), otherName, want)
	}

	return got.String()
}

func TestPrettyWriter_Golden(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		stream  stream
		call    func(w *output.PrettyWriter)
	}{
		{"pretty_info_plain", goldenPlain, streamStderr, func(w *output.PrettyWriter) { w.Info("hello %s", "world") }},
		{"pretty_info_colour", goldenColour, streamStderr, func(w *output.PrettyWriter) { w.Info("hello %s", "world") }},
		{"pretty_warn_plain", goldenPlain, streamStderr, func(w *output.PrettyWriter) { w.Warn("something wrong") }},
		{"pretty_warn_colour", goldenColour, streamStderr, func(w *output.PrettyWriter) { w.Warn("something wrong") }},
		{"pretty_error_plain", goldenPlain, streamStderr, func(w *output.PrettyWriter) { w.Error("fatal error") }},
		{"pretty_error_colour", goldenColour, streamStderr, func(w *output.PrettyWriter) { w.Error("fatal error") }},
		{
			"pretty_writeerr_sentinel_plain", goldenPlain, streamStderr,
			func(w *output.PrettyWriter) { w.WriteErr(specs.ErrTemplateNotFound) },
		},
		{
			"pretty_writeerr_sentinel_colour", goldenColour, streamStderr,
			func(w *output.PrettyWriter) { w.WriteErr(specs.ErrTemplateNotFound) },
		},
		{
			"pretty_writeerr_unknown_plain", goldenPlain, streamStderr,
			func(w *output.PrettyWriter) { w.WriteErr(errors.New("something unexpected")) },
		},
		{
			"pretty_table_plain", goldenPlain, streamStdout,
			func(w *output.PrettyWriter) {
				w.Table([]string{"Name", "Version"}, [][]string{{"my-tpl", "1.0.0"}, {"other", "2.0.0"}})
			},
		},
		{
			"pretty_table_colour", goldenColour, streamStdout,
			func(w *output.PrettyWriter) {
				w.Table([]string{"Name", "Version"}, [][]string{{"my-tpl", "1.0.0"}, {"other", "2.0.0"}})
			},
		},
		{
			// The empty answer a command like `template list` writes when it has
			// nothing to report: headers only, same shape as a filled table.
			"pretty_table_empty", goldenPlain, streamStdout,
			func(w *output.PrettyWriter) { w.Table([]string{"Name", "Version"}, nil) },
		},
		{
			// A result carries no level prefix and no styling: it is the product,
			// not narration about it.
			"pretty_writeresult_plain", goldenPlain, streamStdout,
			func(w *output.PrettyWriter) {
				w.WriteResult(map[string]string{"version": "v1.2.3"}, "specs version %s", "v1.2.3")
			},
		},
		{
			"pretty_writeresult_colour", goldenColour, streamStdout,
			func(w *output.PrettyWriter) {
				w.WriteResult(map[string]string{"version": "v1.2.3"}, "specs version %s", "v1.2.3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer

			tt.call(output.NewPrettyWriter(&out, &errOut, tt.environ))
			assertGolden(t, tt.name, capture(t, &out, &errOut, tt.stream))
		})
	}
}

func TestJSONWriter_Golden(t *testing.T) {
	tests := []struct {
		name   string
		stream stream
		call   func(w *output.JSONWriter)
	}{
		{"json_info", streamStderr, func(w *output.JSONWriter) { w.Info("hello %s", "world") }},
		{"json_warn", streamStderr, func(w *output.JSONWriter) { w.Warn("something wrong") }},
		{"json_error", streamStderr, func(w *output.JSONWriter) { w.Error("fatal error") }},
		{
			"json_writeerr_sentinel", streamStderr,
			func(w *output.JSONWriter) { w.WriteErr(specs.ErrTemplateNotFound) },
		},
		{
			"json_writeerr_unknown", streamStderr,
			func(w *output.JSONWriter) { w.WriteErr(errors.New("something unexpected")) },
		},
		{
			// Emitting one array rather than one object per line is what issue
			// #109 will change.
			"json_table", streamStdout,
			func(w *output.JSONWriter) {
				w.Table([]string{"Name", "Version"}, [][]string{{"my-tpl", "1.0.0"}, {"other", "2.0.0"}})
			},
		},
		{
			"json_table_empty", streamStdout,
			func(w *output.JSONWriter) { w.Table([]string{"Name", "Version"}, nil) },
		},
		{
			// A row shorter than the headers leaves the missing keys out.
			"json_table_ragged", streamStdout,
			func(w *output.JSONWriter) {
				w.Table([]string{"Name", "Version"}, [][]string{{"my-tpl"}, {"other", "2.0.0", "extra"}})
			},
		},
		{
			// The record is marshalled and the sentence dropped, so a consumer
			// reads a field instead of parsing English.
			"json_writeresult", streamStdout,
			func(w *output.JSONWriter) {
				w.WriteResult(map[string]string{"version": "v1.2.3"}, "specs version %s", "v1.2.3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer

			tt.call(output.NewJSONWriter(&out, &errOut))
			assertGolden(t, tt.name, capture(t, &out, &errOut, tt.stream))
		})
	}
}

// Every JSON line is framed by exactly one trailing newline, so consecutive
// calls stay parseable as NDJSON.
func TestJSONWriter_LineFraming(t *testing.T) {
	var out bytes.Buffer

	w := output.NewJSONWriter(&out, &bytes.Buffer{})
	w.WriteResult(map[string]string{"first": "1"}, "first")
	w.WriteResult(map[string]string{"second": "2"}, "second")

	lines := strings.Split(out.String(), "\n")
	if len(lines) != 3 || lines[2] != "" {
		t.Fatalf("expected two newline-terminated lines, got %q", out.String())
	}

	for _, line := range lines[:2] {
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			t.Errorf("line is not a bare JSON object: %q", line)
		}
	}
}

// A record JSON cannot represent writes nothing at all, rather than a broken
// line into a stream a consumer is parsing.
func TestJSONWriter_WriteResultSkipsUnmarshalableRecord(t *testing.T) {
	var out, errOut bytes.Buffer

	output.NewJSONWriter(&out, &errOut).WriteResult(func() {}, "unreachable")

	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("wrote %q / %q, expected nothing", out.String(), errOut.String())
	}
}

// JSON output is never styled, whatever the environment says.
func TestJSONWriter_NeverStyled(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")

	var out, errOut bytes.Buffer

	w := output.NewJSONWriter(&out, &errOut)
	w.Info("hello")
	w.Error("bad")

	if strings.Contains(out.String()+errOut.String(), esc) {
		t.Errorf("JSON output contains escape sequences: %q %q", out.String(), errOut.String())
	}
}

func TestPrettyWriter_ColourFollowsEnviron(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		want    string
	}{
		{
			name:    "empty environ strips every sequence",
			environ: goldenPlain,
			want:    "info hello\n",
		},
		{
			name:    "CLICOLOR_FORCE renders colour to a non-terminal",
			environ: goldenColour,
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

			output.NewPrettyWriter(&bytes.Buffer{}, &buf, tt.environ).Info("hello")

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

	output.NewPrettyWriter(&bytes.Buffer{}, &buf, goldenPlain).Info("hello")

	if strings.Contains(buf.String(), esc) {
		t.Errorf("process environment leaked into the rendering: %q", buf.String())
	}
}

// Each stream gets its own colorprofile.Writer, so a decision made for one is
// not inherited by the other.
func TestPrettyWriter_StreamsAreWrappedIndependently(t *testing.T) {
	var out, errOut bytes.Buffer

	w := output.NewPrettyWriter(&out, &errOut, goldenColour)
	w.Table([]string{"Name"}, [][]string{{"my-tpl"}})
	w.Warn("warn")

	if !strings.Contains(out.String(), esc) {
		t.Errorf("stdout not styled: %q", out.String())
	}

	if !strings.Contains(errOut.String(), esc) {
		t.Errorf("stderr not styled: %q", errOut.String())
	}
}
