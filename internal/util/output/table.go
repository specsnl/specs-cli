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

// Cell is one table cell. Value is what a consumer parses and what JSONWriter
// emits; Text is what a reader sees, falling back to Value when empty; Link,
// when set, is the URL Text is hyperlinked to.
//
// The split exists because a readable label is not always the value. A caller
// that knows what a value means — that "~/x" is a path, or that a repository
// URL's host is noise — can shorten the label without touching what a script
// reads back. Cell{Value: x} is the ordinary cell where the two coincide.
type Cell struct {
	Value string
	Text  string
	Link  string
}

// Label is what a reader sees: Text when it is set, otherwise Value. Callers
// that build a Cell for its value alone leave Text empty rather than repeating
// the value in both fields.
func (c Cell) Label() string {
	if c.Text != "" {
		return c.Text
	}

	return c.Value
}

// Rows adapts plain string rows into cells that display their own value, for
// the common table that needs no separate label.
func Rows(rows [][]string) [][]Cell {
	cells := make([][]Cell, len(rows))

	for i, row := range rows {
		cellRow := make([]Cell, len(row))
		for j, v := range row {
			cellRow[j] = Cell{Value: v}
		}

		cells[i] = cellRow
	}

	return cells
}

// RenderTable renders headers and rows as a styled table string.
//
// Column widths come from the rendered text. maxWidth caps the rendered table:
// when the natural table is wider, the widest columns shrink first and data
// cells wrap onto extra lines instead of overflowing the terminal. maxWidth <= 0
// leaves the table unconstrained, which is the right answer for a file or a
// pipe. Detecting the terminal is the caller's job — see PrettyWriter.Table —
// so this function stays environment-free and its goldens deterministic.
//
// Headers are never wrapped, so a maxWidth narrower than the headers themselves
// truncates their text.
//
// A cell with a Link, or whose Value is entirely an http(s) URL, is rendered as
// an OSC 8 hyperlink — one link even when the cap wraps it over several lines.
// See cellLink.
func RenderTable(headers []string, rows [][]Cell, maxWidth int) string {
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
// rendered to its display text, hyperlinked where cellLink says so.
func fitRows(headers []string, rows [][]Cell) [][]string {
	fitted := make([][]string, len(rows))

	for i, row := range rows {
		fittedRow := make([]string, len(headers))

		for j := range fittedRow {
			if j >= len(row) {
				break
			}

			fittedRow[j] = renderCell(row[j], i, j)
		}

		fitted[i] = fittedRow
	}

	return fitted
}

// renderCell returns a cell's display text, wrapped in an OSC 8 hyperlink when
// it has a link target.
//
// The id parameter is what makes a *wrapped* cell one link rather than several:
// per the OSC 8 spec, segments sharing an id are one logical link, so hovering
// highlights every line of the cell and a click anywhere in it opens the whole
// URL. row/col makes it unique per cell, so two rows pointing at the same
// repository still highlight separately.
//
// This is deliberately not gated on the terminal: the sequences reach a stream
// that cannot use them only to be stripped by its colorprofile.Writer, exactly
// as bold and colour already are. See PrettyWriter.
func renderCell(cell Cell, row, col int) string {
	text := cell.Label()

	link := cellLink(cell)
	if link == "" {
		return text
	}

	id := fmt.Sprintf("id=%d-%d", row, col)

	return ansi.SetHyperlink(link, id) + text + ansi.ResetHyperlink()
}

// cellLink resolves what a cell links to, or "" for no link.
//
// An explicit Link wins — that is the caller saying "this label stands for that
// URL". Otherwise a Value that is entirely an http(s) URL links to itself, so
// any table gets clickable URLs without its command having to ask. Anything
// else is not linked: an SSH remote (git@host:path) and a filesystem path are
// not things a terminal can open.
func cellLink(cell Cell) string {
	if cell.Link != "" {
		return cell.Link
	}

	if !strings.HasPrefix(cell.Value, "https://") && !strings.HasPrefix(cell.Value, "http://") {
		return ""
	}

	// A cell holding a sentence that merely starts with a URL is prose, not a
	// link target, and the URL would not survive round-tripping anyway.
	if strings.ContainsAny(cell.Value, " \t\n") {
		return ""
	}

	return cell.Value
}
