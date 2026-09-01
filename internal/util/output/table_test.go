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
		rendered := output.RenderTable(wideHeaders, wideRows, maxWidth)

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
	natural := output.RenderTable(wideHeaders, wideRows, 0)
	capped := output.RenderTable(wideHeaders, wideRows, 40)

	if lipgloss.Height(capped) <= lipgloss.Height(natural) {
		t.Errorf("capped table did not grow taller:\n%s", capped)
	}
}

// A cap at or above the natural width changes nothing: the table is never
// stretched to fill the terminal, and maxWidth <= 0 means unconstrained.
func TestRenderTable_NeverStretches(t *testing.T) {
	natural := output.RenderTable(wideHeaders, wideRows, 0)

	for _, maxWidth := range []int{lipgloss.Width(natural), lipgloss.Width(natural) + 40, -1} {
		if got := output.RenderTable(wideHeaders, wideRows, maxWidth); got != natural {
			t.Errorf("maxWidth %d changed the rendering:\n%s", maxWidth, got)
		}
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
				[][]string{{tt.wide, "x"}, {"abcdefgh", "y"}},
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
