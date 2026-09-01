package output

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

var (
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	tableCellStyle   = lipgloss.NewStyle().Padding(0, 1)
	tableBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(240))
)

// RenderTable renders headers and rows as a styled table string.
//
// Column widths come from the content. maxWidth caps the rendered table: when
// the natural table is wider, the widest columns shrink first and data cells
// wrap onto extra lines instead of overflowing the terminal. maxWidth <= 0
// leaves the table unconstrained, which is the right answer for a file or a
// pipe. Detecting the terminal is the caller's job — see PrettyWriter.Table —
// so this function stays environment-free and its goldens deterministic.
//
// Headers are never wrapped, so a maxWidth narrower than the headers themselves
// truncates their text.
func RenderTable(headers []string, rows [][]string, maxWidth int) string {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(tableBorderStyle).
		BorderColumn(false).
		BorderRow(false).
		Wrap(true).
		Headers(headers...).
		Rows(fitRows(headers, rows)...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return tableHeaderStyle
			}

			return tableCellStyle
		})

	// Cap only, never stretch: Width() also expands a narrow table to fill the
	// space, so short tables keep their compact look and the output does not
	// change with every terminal size.
	if maxWidth > 0 && lipgloss.Width(t.String()) > maxWidth {
		t.Width(maxWidth)
	}

	return t.String()
}

// fitRows squares rows off to len(headers) columns: a short row is padded with
// empty cells and a long one truncated. The table renderer would otherwise grow
// a headerless extra column for the overlong row.
func fitRows(headers []string, rows [][]string) [][]string {
	fitted := make([][]string, len(rows))

	for i, row := range rows {
		fittedRow := make([]string, len(headers))
		copy(fittedRow, row)
		fitted[i] = fittedRow
	}

	return fitted
}
