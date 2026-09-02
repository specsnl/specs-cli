package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/specsnl/specs-cli/internal/specs"
)

var (
	styleInfo  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(12)) // bright blue
	styleWarn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(11)) // bright yellow
	styleError = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(9))  // bright red
)

// Writer is the interface for all user-facing output.
//
// stdout carries the product — what a caller would redirect to a file or pipe
// into another command — and stderr carries the narration. Table and WriteResult
// write the product; Info, Warn, Error and WriteErr narrate.
type Writer interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	// WriteErr renders err as an error-level message. JSON output includes an
	// "error_kind" field when err wraps a known specs sentinel.
	WriteErr(err error)
	// Table renders rows under headers. A Cell's Value is what JSON emits and
	// its Text what the pretty writer displays, so a command can shorten a
	// label without changing what a script reads back. Wrap plain string rows
	// with Rows.
	Table(headers []string, rows [][]Cell)
	// WriteResult renders a single-line result on stdout: the product of a command
	// whose answer is not a table. Pretty writes the formatted text; JSON marshals
	// record and ignores the text, so every stdout line stays a typed object.
	WriteResult(record any, format string, args ...any)
}

// PrettyWriter writes lipgloss-styled output to stdout/stderr.
type PrettyWriter struct {
	stdout io.Writer
	stderr io.Writer

	// rawStdout is stdout before the colorprofile wrapper, kept so the table
	// width can be read from its file descriptor; environ is the captured
	// environment COLUMNS is read from. Both feed tableWidth.
	rawStdout io.Writer
	environ   []string
}

// NewPrettyWriter creates a PrettyWriter over the given streams. Each stream is
// wrapped in a colorprofile.Writer, so how much colour is emitted is decided from
// that stream and the given environment rather than from the process as a whole.
// NO_COLOR, CLICOLOR and CLICOLOR_FORCE are honoured. Pass nil for environ to use
// the process environment; tests pass an explicit slice so the output does not
// depend on who is running them.
func NewPrettyWriter(stdout, stderr io.Writer, environ []string) *PrettyWriter {
	// colorprofile.NewWriter documents a nil environ as "use os.Environ()" but
	// builds an empty environment from it, which would strip colour everywhere.
	if environ == nil {
		environ = os.Environ()
	}

	return &PrettyWriter{
		stdout:    profileWriter(stdout, environ),
		stderr:    profileWriter(stderr, environ),
		rawStdout: stdout,
		environ:   environ,
	}
}

// profileWriter wraps w so that colour is downsampled to what that stream and
// environ support.
//
// NO_COLOR is applied here rather than left to colorprofile, which honours it
// only for a stream that is itself a terminal — so NO_COLOR=1 together with
// CLICOLOR_FORCE=1 would otherwise still emit colour into a pipe. ASCII keeps
// bold and the other text decorations, which is what no-color.org asks for.
func profileWriter(w io.Writer, environ []string) io.Writer {
	pw := colorprofile.NewWriter(w, environ)
	if envNoColor(environ) && pw.Profile > colorprofile.ASCII {
		pw.Profile = colorprofile.ASCII
	}

	return pw
}

// envNoColor reports whether environ sets NO_COLOR, using the same boolean
// parsing colorprofile applies.
func envNoColor(environ []string) bool {
	for _, e := range environ {
		name, value, found := strings.Cut(e, "=")
		if found && name == "NO_COLOR" {
			noColor, _ := strconv.ParseBool(value)

			return noColor
		}
	}

	return false
}

// NewDefaultPrettyWriter creates a PrettyWriter over os.Stdout/os.Stderr using the
// process environment.
func NewDefaultPrettyWriter() *PrettyWriter {
	return NewPrettyWriter(os.Stdout, os.Stderr, nil)
}

// The streams already downsample colour, so these render with fmt rather than
// lipgloss.Fprintln — the latter wraps the stream again using the process
// environment, which is the decision this writer exists to avoid.

func (w *PrettyWriter) Info(format string, args ...any) {
	fmt.Fprintln(w.stderr, fmt.Sprintf(styleInfo.Render("info")+" "+format, args...))
}

func (w *PrettyWriter) Warn(format string, args ...any) {
	fmt.Fprintln(w.stderr, fmt.Sprintf(styleWarn.Render("warn")+" "+format, args...))
}

func (w *PrettyWriter) Error(format string, args ...any) {
	fmt.Fprintln(w.stderr, fmt.Sprintf(styleError.Render("error")+" "+format, args...))
}

func (w *PrettyWriter) WriteErr(err error) {
	w.Error("%v", err)
}

func (w *PrettyWriter) Table(headers []string, rows [][]Cell) {
	fmt.Fprintln(w.stdout, RenderTable(headers, rows, w.tableWidth()))
}

// tableWidth resolves the width a table may occupy, resolved per call so a
// terminal resized between two commands is honoured: the size of stdout when it
// is a terminal, else a positive COLUMNS from the captured environment — the
// escape hatch for pipes and tests — else 0 for unconstrained, which is what a
// file or a pipe wants.
func (w *PrettyWriter) tableWidth() int {
	if f, ok := w.rawStdout.(interface{ Fd() uintptr }); ok {
		fd := f.Fd()
		if term.IsTerminal(fd) {
			if width, _, err := term.GetSize(fd); err == nil && width > 0 {
				return width
			}
		}
	}

	return envColumns(w.environ)
}

// envColumns returns a positive COLUMNS from environ, or 0 when it is unset,
// unparseable or not positive.
func envColumns(environ []string) int {
	for _, e := range environ {
		name, value, found := strings.Cut(e, "=")
		if !found || name != "COLUMNS" {
			continue
		}

		if columns, err := strconv.Atoi(value); err == nil && columns > 0 {
			return columns
		}

		return 0
	}

	return 0
}

// The result is unprefixed and unstyled: it is an answer, not a level.
func (w *PrettyWriter) WriteResult(_ any, format string, args ...any) {
	fmt.Fprintln(w.stdout, fmt.Sprintf(format, args...))
}

// JSONWriter writes NDJSON output: table/result to stdout, info/warn/error to
// stderr.
type JSONWriter struct {
	stdout io.Writer
	stderr io.Writer
}

// NewJSONWriter creates a JSONWriter writing to the given streams.
func NewJSONWriter(stdout, stderr io.Writer) *JSONWriter {
	return &JSONWriter{stdout: stdout, stderr: stderr}
}

func (w *JSONWriter) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	data, _ := json.Marshal(map[string]string{"level": "info", "message": msg})
	fmt.Fprintln(w.stderr, string(data))
}

func (w *JSONWriter) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	data, _ := json.Marshal(map[string]string{"level": "warn", "message": msg})
	fmt.Fprintln(w.stderr, string(data))
}

func (w *JSONWriter) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	data, _ := json.Marshal(map[string]string{"level": "error", "message": msg})
	fmt.Fprintln(w.stderr, string(data))
}

func (w *JSONWriter) WriteErr(err error) {
	payload := map[string]string{
		"level":   "error",
		"message": err.Error(),
	}
	if kind := specs.KindOf(err); kind != "" {
		payload["error_kind"] = kind
	}

	data, _ := json.Marshal(payload)
	fmt.Fprintln(w.stderr, string(data))
}

// Table outputs an array of JSON objects, one per row, keyed by column header.
//
// Only a cell's Value is emitted: Text is a label for a reader and Link is a
// terminal capability, neither of which belongs in a stream a consumer parses.
func (w *JSONWriter) Table(headers []string, rows [][]Cell) {
	records := make([]map[string]string, len(rows))

	for i, row := range rows {
		record := make(map[string]string, len(headers))

		for j, header := range headers {
			if j < len(row) {
				record[header] = row[j].Value
			}
		}

		records[i] = record
	}

	data, _ := json.Marshal(records)
	fmt.Fprintln(w.stdout, string(data))
}

// The formatted text is dropped: a consumer reads the record's fields rather
// than parsing an English sentence back apart.
func (w *JSONWriter) WriteResult(record any, _ string, _ ...any) {
	data, err := json.Marshal(record)
	if err != nil {
		return
	}

	fmt.Fprintln(w.stdout, string(data))
}
