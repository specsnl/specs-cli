package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
)

var (
	styleInfo  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(12)) // bright blue
	styleWarn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(11)) // bright yellow
	styleError = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(9))  // bright red
)

// Writer is the interface for all user-facing output.
type Writer interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	Table(headers []string, rows [][]string)
}

// HumanWriter writes lipgloss-styled output to stdout/stderr.
type HumanWriter struct {
	stdout io.Writer
	stderr io.Writer
}

// NewHumanWriter creates a HumanWriter writing to the given streams.
func NewHumanWriter(stdout, stderr io.Writer) *HumanWriter {
	return &HumanWriter{stdout: stdout, stderr: stderr}
}

// NewDefaultHumanWriter creates a HumanWriter writing to os.Stdout/os.Stderr.
func NewDefaultHumanWriter() *HumanWriter {
	return NewHumanWriter(os.Stdout, os.Stderr)
}

func (w *HumanWriter) Info(format string, args ...any) {
	lipgloss.Fprintln(w.stdout, fmt.Sprintf(styleInfo.Render("info")+" "+format, args...))
}

func (w *HumanWriter) Warn(format string, args ...any) {
	lipgloss.Fprintln(w.stderr, fmt.Sprintf(styleWarn.Render("warn")+" "+format, args...))
}

func (w *HumanWriter) Error(format string, args ...any) {
	lipgloss.Fprintln(w.stderr, fmt.Sprintf(styleError.Render("error")+" "+format, args...))
}

func (w *HumanWriter) Table(headers []string, rows [][]string) {
	fmt.Fprintln(w.stdout, RenderTable(headers, rows))
}

// JSONWriter writes NDJSON output: info/table to stdout, warn/error to stderr.
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
	fmt.Fprintln(w.stdout, string(data))
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

// Table outputs an array of JSON objects, one per row, keyed by column header.
func (w *JSONWriter) Table(headers []string, rows [][]string) {
	records := make([]map[string]string, len(rows))
	for i, row := range rows {
		record := make(map[string]string, len(headers))
		for j, header := range headers {
			if j < len(row) {
				record[header] = row[j]
			}
		}
		records[i] = record
	}
	data, _ := json.Marshal(records)
	fmt.Fprintln(w.stdout, string(data))
}
