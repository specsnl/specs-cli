package output

// Column describes one column of a table: the heading a human reads, and how a
// row renders in that column.
//
// A heading is prose for a reader and may be reworded freely. It is deliberately
// not what names the field in --output json — that comes from the row type's
// json tags, so rewording "Updates available" cannot silently start returning
// null from a consumer's jq filter.
type Column[T any] struct {
	Header string
	Cell   func(T) Cell
}

// Col builds a column whose cell displays its own value, which is the ordinary
// case.
func Col[T any](header string, value func(T) string) Column[T] {
	return Column[T]{
		Header: header,
		Cell:   func(row T) Cell { return Cell{Value: value(row)} },
	}
}

// ColCell builds a column whose cell carries a separate label or link — a
// shortened repository, say. See Cell.
func ColCell[T any](header string, cell func(T) Cell) Column[T] {
	return Column[T]{Header: header, Cell: cell}
}

// TableData is one table, in the two forms its two audiences need: Headers and
// Cells for the reader, Records for the consumer.
//
// Keeping them apart is what lets a number stay a number. Cells hold display
// strings because that is what a bordered table is made of; Records hold the row
// values themselves, so JSON emits 12 rather than "12" and a timestamp rather
// than "3 days ago".
type TableData struct {
	Headers []string
	Cells   [][]Cell
	// Records is one row value per row of Cells, marshalled one per line by the
	// JSON writer.
	Records []any
}

// Table writes rows as the command's product: a bordered table for a reader, one
// JSON object per line for a consumer.
//
// It is a function rather than a Writer method because Go has no generic
// methods. Going through it is also what guarantees every row has exactly one
// cell per header — alignment a raw [][]Cell can silently get wrong.
func Table[T any](w Writer, rows []T, cols ...Column[T]) {
	data := TableData{
		Headers: make([]string, len(cols)),
		Cells:   make([][]Cell, len(rows)),
		Records: make([]any, len(rows)),
	}

	for i, col := range cols {
		data.Headers[i] = col.Header
	}

	for i, row := range rows {
		cells := make([]Cell, len(cols))
		for j, col := range cols {
			cells[j] = col.Cell(row)
		}

		data.Cells[i] = cells
		data.Records[i] = row
	}

	w.WriteTable(data)
}
