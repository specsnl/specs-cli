package output

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	tableCellStyle   = lipgloss.NewStyle().Padding(0, 1)
	tableBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(240))
)

// RenderTable renders headers and rows as a styled table string.
// Column widths are computed automatically from content.
func RenderTable(headers []string, rows [][]string) string {
	widths := columnWidths(headers, rows)

	var sb strings.Builder

	// Header row
	for i, h := range headers {
		sb.WriteString(tableHeaderStyle.Width(widths[i] + 2).Render(h))
	}
	sb.WriteString("\n")

	// Separator
	for _, w := range widths {
		sb.WriteString(tableBorderStyle.Render(strings.Repeat("─", w+2))) // +2 for padding
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			sb.WriteString(tableCellStyle.Width(widths[i] + 2).Render(cell))
		}
		sb.WriteString("\n")
	}

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.ANSIColor(240)).
		Render(strings.TrimRight(sb.String(), "\n"))
}

func columnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	return widths
}
