package sheetframe

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
)

// ReadCSV reads a header and typed rows. Blank cells are null. Columns that
// contain only numeric or true/false cells are inferred; otherwise they remain
// strings. Numeric inference may remove leading zeros from identifiers.
func ReadCSV(r io.Reader) (Frame, error) {
	reader := csv.NewReader(r)
	headers, err := reader.Read()
	if err != nil {
		return Frame{}, fmt.Errorf("CSV header: %w", err)
	}
	records, err := reader.ReadAll()
	if err != nil {
		return Frame{}, fmt.Errorf("CSV records: %w", err)
	}
	rows := make([][]any, len(records))
	for i := range rows {
		rows[i] = make([]any, len(headers))
	}
	for c := range headers {
		numeric, boolean := true, true
		for _, row := range records {
			if row[c] == "" {
				continue
			}
			if _, err := strconv.ParseFloat(row[c], 64); err != nil {
				numeric = false
			}
			if row[c] != "true" && row[c] != "false" {
				boolean = false
			}
		}
		for i, row := range records {
			s := row[c]
			if s == "" {
				continue
			}
			switch {
			case numeric:
				n, _ := strconv.ParseFloat(s, 64)
				rows[i][c] = n
			case boolean:
				rows[i][c] = s == "true"
			default:
				rows[i][c] = s
			}
		}
	}
	return New(headers, rows)
}

// WriteCSV writes column headers and rows, representing nulls as blank cells.
// It writes values verbatim, including strings that spreadsheet applications
// may interpret as formulas. XLSX export stores strings as literal text instead.
func (f Frame) WriteCSV(w io.Writer) error {
	out := csv.NewWriter(w)
	if err := out.Write(f.Columns()); err != nil {
		return err
	}
	for r := 0; r < f.rows; r++ {
		row := make([]string, len(f.columns))
		for c, col := range f.columns {
			row[c] = formatValue(col.values[r])
		}
		if err := out.Write(row); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

func formatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}
