package output_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/specsnl/specs-cli/internal/util/output"
)

// RenderTable returns a styled string and consults no environment, so each case
// has a single golden. The escape sequences are part of it: the padding, the
// borders and the column widths are exactly what these files exist to pin.
func TestRenderTable_Golden(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		rows     [][]string
		maxWidth int
	}{
		{
			name:    "table_single_row",
			headers: []string{"Tag", "Repository", "Created"},
			rows:    [][]string{{"my-tag", "user/repo", "2 days ago"}},
		},
		{
			name:    "table_multiple_rows",
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"alpha", "1"}, {"beta", "2"}},
		},
		{
			name:    "table_no_rows",
			headers: []string{"Name", "Value"},
			rows:    nil,
		},
		{
			// A row shorter than the headers ends early; a longer one is
			// truncated to the header count.
			name:    "table_ragged_rows",
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"alpha"}, {"beta", "2", "ignored"}},
		},
		{
			// Cells are measured by display width, not by bytes: "café" is 5
			// bytes but 4 cells wide, so the Name column is exactly as wide as
			// the longest cell in it and the columns line up.
			name:    "table_multibyte_cell",
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"café", "naïve"}, {"ascii", "plain"}},
		},
		{
			// Display-width measurement at its widest: "日本語" is 9 bytes but
			// occupies 6 cells, so it is "abcdefg" that sizes the Name column.
			name:    "table_wide_runes",
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"日本語", "x"}, {"abcdefg", "y"}},
		},
		{
			name:    "table_cell_wider_than_header",
			headers: []string{"N", "V"},
			rows:    [][]string{{"a-much-longer-value", "x"}},
		},
		{
			// Capped well below the natural width: the widest column shrinks and
			// its cells wrap onto extra lines rather than the border overflowing.
			name:     "table_width_capped",
			headers:  []string{"Name", "Repository"},
			rows:     [][]string{{"my-tpl", "https://github.com/specsnl/specs-cli.git"}},
			maxWidth: 30,
		},
		{
			// A cap wider than the table leaves it alone — no stretching to fill.
			name:     "table_width_above_natural",
			headers:  []string{"Name", "Value"},
			rows:     [][]string{{"alpha", "1"}, {"beta", "2"}},
			maxWidth: 200,
		},
		{
			// An http(s) cell becomes an OSC 8 hyperlink; an SSH remote and a
			// local: path are left alone, being nothing a terminal can open.
			name:    "table_hyperlink",
			headers: []string{"Name", "Repository"},
			rows: [][]string{
				{"remote", "https://github.com/specsnl/specs-cli.git"},
				{"ssh", "git@github.com:specsnl/specs-cli.git"},
				{"local", "local:/home/me/templates/go"},
			},
		},
		{
			// Wrapped over three lines, each segment carries the same id, so the
			// terminal treats them as one link — and each closes before the
			// padding, so the border is never part of it.
			name:     "table_hyperlink_wrapped",
			headers:  []string{"Name", "Repository"},
			rows:     [][]string{{"my-tpl", "https://github.com/specsnl/specs-cli.git"}},
			maxWidth: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertGolden(t, tt.name, output.RenderTable(tt.headers, output.Rows(tt.rows), tt.maxWidth))
		})
	}
}

// Cases that need a Cell's Text or Link, which Rows cannot express.
func TestRenderTable_CellGolden(t *testing.T) {
	url := "https://github.com/specsnl/specs-cli"

	tests := []struct {
		name     string
		headers  []string
		rows     [][]output.Cell
		maxWidth int
	}{
		{
			// The shape template list produces: a short label carrying the full
			// URL as its target, an unlinked path, and a placeholder.
			name:    "table_shortened_labels",
			headers: []string{"Name", "Repository", "Version"},
			rows: [][]output.Cell{
				{{Value: "laravel-api"}, {Value: url, Text: "specsnl/specs-cli", Link: url}, {Value: "1.2.0"}},
				{{Value: "internal-web"}, {Value: "git@git.acme.internal:web/starter"}, {Value: "2.0.0"}},
				{{Value: "local-proto"}, {Value: "local:/Users/me/code/proto", Text: "~/code/proto"}, {Value: "-"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertGolden(t, tt.name, output.RenderTable(tt.headers, tt.rows, tt.maxWidth))
		})
	}
}

var wideRows = [][]string{
	{"my-tpl", "https://github.com/specsnl/specs-cli.git", "an unusually long description"},
	{"other", "git@github.com:specsnl/some-other-template.git", "another long description"},
}

var wideHeaders = []string{"Name", "Repository", "Description"}

// A cap is a hard limit: no rendered line may exceed it, which is the defect
// that made a narrow terminal wrap the border instead of the cells.
func TestRenderTable_CapsEveryLineToMaxWidth(t *testing.T) {
	for _, maxWidth := range []int{40, 60, 70} {
		rendered := output.RenderTable(wideHeaders, output.Rows(wideRows), maxWidth)

		for i, line := range strings.Split(rendered, "\n") {
			if got := lipgloss.Width(line); got > maxWidth {
				t.Errorf("maxWidth %d: line %d is %d cells wide: %q", maxWidth, i+1, got, line)
			}
		}
	}
}

// Capping shrinks columns and wraps their cells, so the table grows taller
// rather than losing content off the right edge.
func TestRenderTable_WrapsInsteadOfTruncating(t *testing.T) {
	natural := output.RenderTable(wideHeaders, output.Rows(wideRows), 0)
	capped := output.RenderTable(wideHeaders, output.Rows(wideRows), 40)

	if lipgloss.Height(capped) <= lipgloss.Height(natural) {
		t.Errorf("capped table did not grow taller:\n%s", capped)
	}
}

// A cap at or above the natural width changes nothing: the table is never
// stretched to fill the terminal, and maxWidth <= 0 means unconstrained.
func TestRenderTable_NeverStretches(t *testing.T) {
	natural := output.RenderTable(wideHeaders, output.Rows(wideRows), 0)

	for _, maxWidth := range []int{lipgloss.Width(natural), lipgloss.Width(natural) + 40, -1} {
		if got := output.RenderTable(wideHeaders, output.Rows(wideRows), maxWidth); got != natural {
			t.Errorf("maxWidth %d changed the rendering:\n%s", maxWidth, got)
		}
	}
}

const (
	// openLink is the prefix of an OSC 8 hyperlink; closeLink ends one.
	openLink  = "\x1b]8;id="
	closeLink = "\x1b]8;;\x07"
)

// A URL split over several lines must be one link, not one per line: every
// segment carries the same id, which is what the OSC 8 spec asks for.
func TestRenderTable_WrappedURLIsOneLink(t *testing.T) {
	url := "https://github.com/specsnl/specs-cli.git"

	rendered := output.RenderTable(
		[]string{"Name", "Repository"},
		output.Rows([][]string{{"my-tpl", url}}),
		30,
	)

	// The cap is well below the URL, so it has to have wrapped.
	segments := strings.Count(rendered, openLink)
	if segments < 2 {
		t.Fatalf("expected the URL to wrap into several segments, got %d:\n%s", segments, rendered)
	}

	// One id, repeated — and the full URL as the target of every segment.
	if got := strings.Count(rendered, openLink+"0-1;"+url); got != segments {
		t.Errorf("%d of %d segments carry id=0-1 and the full URL:\n%s", got, segments, rendered)
	}

	// Each segment closes before the padding and the border, so the border is
	// never swallowed into the link.
	if got := strings.Count(rendered, closeLink); got != segments {
		t.Errorf("%d segments opened but %d closed:\n%s", segments, got, rendered)
	}
}

// A Cell's Text is what gets rendered and its Link what gets opened, so a short
// label can stand for a long URL. This is what lets a command narrow a column
// without touching the value a script reads.
func TestRenderTable_TextAndLinkAreIndependent(t *testing.T) {
	url := "https://github.com/specsnl/specs-cli"

	rendered := output.RenderTable(
		[]string{"Name", "Repository"},
		[][]output.Cell{{
			{Value: "my-tpl"},
			{Value: url, Text: "specsnl/specs-cli", Link: url},
		}},
		0,
	)

	if !strings.Contains(rendered, "specsnl/specs-cli") || strings.Contains(rendered, "│ https://") {
		t.Errorf("the label is not what was rendered:\n%s", rendered)
	}

	if !strings.Contains(rendered, openLink+"0-1;"+url) {
		t.Errorf("the label does not link to the URL:\n%s", rendered)
	}

	// Sized to the label, not to the value it stands for — the point of the
	// whole exercise. The same table built from the label as a plain value is
	// the yardstick.
	plain := output.RenderTable(
		[]string{"Name", "Repository"},
		output.Rows([][]string{{"my-tpl", "specsnl/specs-cli"}}),
		0,
	)

	if got, want := lipgloss.Width(rendered), lipgloss.Width(plain); got != want {
		t.Errorf("table is %d cells wide, want %d — not sized to the label:\n%s", got, want, rendered)
	}
}

// An explicit Link wins over the auto-detection, so a cell can point somewhere
// other than at its own text.
func TestRenderTable_ExplicitLinkOverridesAutoDetection(t *testing.T) {
	rendered := output.RenderTable(
		[]string{"Repository"},
		[][]output.Cell{{{
			Value: "https://github.com/specsnl/specs-cli.git",
			Link:  "https://github.com/specsnl/specs-cli",
		}}},
		0,
	)

	if !strings.Contains(rendered, openLink+"0-0;https://github.com/specsnl/specs-cli\x07") {
		t.Errorf("the explicit link was not used:\n%s", rendered)
	}
}

// A cell with neither Text nor Link is the ordinary case Rows produces, and
// still auto-links a bare URL.
func TestRenderTable_ValueOnlyCellStillAutoLinks(t *testing.T) {
	url := "https://example.com/x"

	rendered := output.RenderTable(
		[]string{"Repository"},
		[][]output.Cell{{{Value: url}}},
		0,
	)

	if !strings.Contains(rendered, openLink+"0-0;"+url) || !strings.Contains(rendered, url) {
		t.Errorf("a value-only cell was not auto-linked:\n%s", rendered)
	}
}

// Two rows pointing at the same repository get different ids, so hovering one
// does not highlight the other.
func TestRenderTable_LinkIDsArePerCell(t *testing.T) {
	url := "https://github.com/specsnl/specs-cli.git"

	rendered := output.RenderTable(
		[]string{"Name", "Repository"},
		output.Rows([][]string{{"a", url}, {"b", url}}),
		0,
	)

	for _, id := range []string{"0-1", "1-1"} {
		if !strings.Contains(rendered, openLink+id+";"+url) {
			t.Errorf("no segment with id=%s:\n%s", id, rendered)
		}
	}
}

// Only an http(s) cell is a link. A header is never one, and neither is
// anything a terminal could not open.
func TestRenderTable_LinksOnlyHTTPDataCells(t *testing.T) {
	tests := []struct {
		name string
		cell string
		want bool
	}{
		{"https URL", "https://github.com/specsnl/specs-cli.git", true},
		{"http URL", "http://example.com/x", true},
		{"SSH remote", "git@github.com:specsnl/specs-cli.git", false},
		{"local path", "local:/home/me/templates/go", false},
		{"prose containing a URL", "https://example.com is the source", false},
		{"placeholder", "-", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := output.RenderTable([]string{"Name", "Value"}, output.Rows([][]string{{"row", tt.cell}}), 0)

			if got := strings.Contains(rendered, openLink); got != tt.want {
				t.Errorf("linked = %v, want %v:\n%s", got, tt.want, rendered)
			}
		})
	}

	// A header that is itself a URL stays plain: a column label is not a target.
	rendered := output.RenderTable([]string{"https://example.com"}, output.Rows([][]string{{"row"}}), 0)
	if strings.Contains(rendered, openLink) {
		t.Errorf("header was linked:\n%s", rendered)
	}
}

// The escape bytes of a hyperlink are zero-width, so a linked cell sizes its
// column exactly as the bare URL would. Without this, one link would blow the
// layout out by ~90 columns.
func TestRenderTable_HyperlinkDoesNotAffectWidth(t *testing.T) {
	headers := []string{"Name", "Repository"}
	linked := [][]string{{"my-tpl", "https://github.com/specsnl/specs-cli.git"}}
	// Same text, one character changed so it is not recognised as a URL.
	plain := [][]string{{"my-tpl", "https:/xgithub.com/specsnl/specs-cli.git"}}

	if got, want := lipgloss.Width(output.RenderTable(headers, output.Rows(linked), 0)),
		lipgloss.Width(output.RenderTable(headers, output.Rows(plain), 0)); got != want {
		t.Errorf("linked table is %d cells wide, the same table unlinked is %d", got, want)
	}
}

// Columns are sized by display width, so a cell that is wide in bytes but
// narrow on screen does not push its column out. This is the case that fails if
// measurement ever reverts to len().
func TestRenderTable_MeasuresByDisplayWidth(t *testing.T) {
	tests := []struct {
		name string
		wide string
	}{
		{"combining accents", "café"},
		{"CJK", "日本語"},
		{"emoji", "🚀"},
		{"ANSI escapes", "\x1b[1mbold\x1b[m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// "abcdefgh" is wider on screen than any of the cells above, so it
			// alone decides the column width and every line has one length.
			rendered := output.RenderTable(
				[]string{"Name", "Value"},
				output.Rows([][]string{{tt.wide, "x"}, {"abcdefgh", "y"}}),
				0,
			)

			lines := strings.Split(rendered, "\n")
			for i, line := range lines {
				if got, want := lipgloss.Width(line), lipgloss.Width(lines[0]); got != want {
					t.Errorf("line %d is %d cells wide, want %d: %q", i+1, got, want, line)
				}
			}
		})
	}
}
