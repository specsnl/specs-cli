package output_test

import (
	"testing"

	"github.com/specsnl/specs-cli/internal/util/output"
)

// RenderTable returns a styled string and consults no environment, so each case
// has a single golden. The escape sequences are part of it: the padding, the
// borders and the column widths are exactly what these files exist to pin.
func TestRenderTable_Golden(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
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
			// Column widths are computed in bytes while lipgloss pads by display
			// width, so a multibyte cell over-measures its column: "café" counts
			// as 5 and "naïve" as 6. Frozen deliberately — issue #117 changes
			// this, and this golden is its diff.
			name:    "table_multibyte_cell",
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"café", "naïve"}, {"ascii", "plain"}},
		},
		{
			// The same defect at its widest: "日本語" is 9 bytes but occupies 7
			// display cells, so the Name column is padded two cells wider than
			// anything in it. Also #117.
			name:    "table_wide_runes",
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"日本語", "x"}, {"abcdefg", "y"}},
		},
		{
			name:    "table_cell_wider_than_header",
			headers: []string{"N", "V"},
			rows:    [][]string{{"a-much-longer-value", "x"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertGolden(t, tt.name, output.RenderTable(tt.headers, tt.rows))
		})
	}
}
