package output_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/util/output"
)

type tplRow struct {
	Name    string `json:"name"`
	Updates int    `json:"updates_available"`
	Current bool   `json:"up_to_date"`
}

func nameCol() output.Column[tplRow] {
	return output.Col("Name", func(r tplRow) string { return r.Name })
}

func updatesCol(header string) output.Column[tplRow] {
	return output.Col(header, func(r tplRow) string { return strconv.Itoa(r.Updates) })
}

// A number stays a number and a boolean a boolean: the display string the table
// renders is not what JSON sees, so `jq 'select(.updates_available > 5)'` works
// without a tonumber first.
func TestTable_JSONKeepsTypes(t *testing.T) {
	var out, errOut bytes.Buffer

	rows := []tplRow{{Name: "my-tpl", Updates: 12, Current: false}}

	output.Table(output.NewJSONWriter(&out, &errOut), rows,
		nameCol(),
		updatesCol("Updates available"),
		output.Col("Up to date", func(r tplRow) string { return strconv.FormatBool(r.Current) }),
	)

	got := strings.TrimSpace(capture(t, &out, &errOut, streamStdout))

	want := `{"name":"my-tpl","updates_available":12,"up_to_date":false}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Rewording a heading is a change to prose for a reader. It must not move a
// consumer's jq filter out from under it.
func TestTable_HeadingIsNotTheKey(t *testing.T) {
	rows := []tplRow{{Name: "my-tpl", Updates: 3}}

	render := func(header string) string {
		var out, errOut bytes.Buffer

		output.Table(output.NewJSONWriter(&out, &errOut), rows, nameCol(), updatesCol(header))

		return out.String()
	}

	if got, want := render("Pending updates"), render("Updates available"); got != want {
		t.Errorf("rewording the heading changed the JSON: %q vs %q", got, want)
	}
}

// One object per line, not one array: a run that is killed partway still leaves
// every completed row readable.
func TestTable_JSONIsNDJSON(t *testing.T) {
	var out, errOut bytes.Buffer

	rows := []tplRow{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	output.Table(output.NewJSONWriter(&out, &errOut), rows, nameCol())

	got := capture(t, &out, &errOut, streamStdout)

	lines := strings.Split(got, "\n")
	if len(lines) != 4 || lines[3] != "" {
		t.Fatalf("expected three newline-terminated lines, got %q", got)
	}

	for _, line := range lines[:3] {
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			t.Errorf("line is not a bare JSON object: %q", line)
		}
	}
}

// No rows is no lines. The pretty writer still draws the headers, so a reader
// sees the shape of the answer either way.
func TestTable_EmptyRows(t *testing.T) {
	var jsonOut, jsonErr bytes.Buffer

	output.Table(output.NewJSONWriter(&jsonOut, &jsonErr), []tplRow(nil), nameCol())

	if jsonOut.Len() != 0 {
		t.Errorf("JSON wrote %q for no rows, want nothing", jsonOut.String())
	}

	var prettyOut, prettyErr bytes.Buffer

	output.Table(output.NewPrettyWriter(&prettyOut, &prettyErr, goldenPlain), []tplRow(nil), nameCol())

	if !strings.Contains(prettyOut.String(), "Name") {
		t.Errorf("pretty dropped the headers for no rows: %q", prettyOut.String())
	}
}

// Going through the generic constructor is what guarantees the alignment a raw
// [][]Cell could silently get wrong: every row has exactly one cell per header.
func TestTable_RowsAreSquare(t *testing.T) {
	var captured output.TableData

	rows := []tplRow{{Name: "a", Updates: 1}, {Name: "b", Updates: 2}}

	output.Table(recordingWriter{data: &captured}, rows, nameCol(), updatesCol("Updates"))

	if len(captured.Records) != len(rows) {
		t.Fatalf("Records = %d, want one per row (%d)", len(captured.Records), len(rows))
	}

	for i, cells := range captured.Cells {
		if len(cells) != len(captured.Headers) {
			t.Errorf("row %d has %d cells, want %d", i, len(cells), len(captured.Headers))
		}
	}
}

// recordingWriter captures the TableData a Table call produces. The narration
// methods are unreachable here and are never called.
type recordingWriter struct {
	output.Writer

	data *output.TableData
}

func (w recordingWriter) WriteTable(data output.TableData) { *w.data = data }
