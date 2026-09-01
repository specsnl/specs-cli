package output

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"
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
//
// A data cell that is entirely an http(s) URL is rendered as an OSC 8
// hyperlink, one link even when the cap wraps it over several lines — see
// linkCell.
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
// a headerless extra column for the overlong row. Each surviving cell is then
// passed through linkCell.
func fitRows(headers []string, rows [][]string) [][]string {
	fitted := make([][]string, len(rows))

	for i, row := range rows {
		fittedRow := make([]string, len(headers))
		copy(fittedRow, row)

		for j, cell := range fittedRow {
			fittedRow[j] = linkCell(cell, i, j)
		}

		fitted[i] = fittedRow
	}

	return fitted
}

// linkCell wraps a cell that is entirely an http(s) URL in an OSC 8 hyperlink,
// so the terminal opens it on a click instead of leaving the reader to select
// and copy it. Anything else is returned untouched — an SSH remote
// (git@host:path) and a `local:` path are not things a terminal can open.
//
// The id parameter is what makes a *wrapped* URL one link rather than several:
// per the OSC 8 spec, segments sharing an id are one logical link, so hovering
// highlights every line of the cell and a click anywhere in it opens the whole
// URL. row/col makes it unique per cell, so two rows pointing at the same
// repository still highlight separately.
//
// This is deliberately not gated on the terminal: the sequences reach a stream
// that cannot use them only to be stripped by its colorprofile.Writer, exactly
// as bold and colour already are. See PrettyWriter.
func linkCell(cell string, row, col int) string {
	if !strings.HasPrefix(cell, "https://") && !strings.HasPrefix(cell, "http://") {
		return cell
	}

	// A cell holding a sentence that merely starts with a URL is prose, not a
	// link target, and the URL would not survive round-tripping anyway.
	if strings.ContainsAny(cell, " \t\n") {
		return cell
	}

	id := fmt.Sprintf("id=%d-%d", row, col)

	return ansi.SetHyperlink(cell, id) + cell + ansi.ResetHyperlink()
}
