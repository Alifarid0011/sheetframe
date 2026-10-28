// Package sheetframe provides immutable, nullable DataFrames for everyday
// spreadsheet analysis. Values are strings, float64 numbers, bools, time.Time,
// or nil. Numeric inputs are normalized to float64 without accepting inexact
// large integers. Operations are eager and keep their input frames unchanged.
package sheetframe

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Kind describes the non-null values in a column.
type Kind string

const (
	String  Kind = "string"
	Number  Kind = "number"
	Bool    Kind = "bool"
	Time    Kind = "time"
	Unknown Kind = "unknown" // No non-null values have been observed.
)

// Field is a column's name and inferred type.
type Field struct {
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
}

type column struct {
	field  Field
	values []any
}

// Frame is an immutable, column-oriented table. Its zero value is an empty table.
// Frames can safely be read concurrently. Input and returned slices are copied.
type Frame struct {
	columns []column
	index   map[string]int
	rows    int
}

// New builds a typed frame from rows. Names must be unique and nonblank, rows
// must have equal widths, and each column must have one non-null type.
// Empty strings remain strings; nil denotes a missing value.
func New(headers []string, rows [][]any) (Frame, error) {
	f := Frame{columns: make([]column, len(headers)), index: make(map[string]int, len(headers)), rows: len(rows)}
	for i, name := range headers {
		if strings.TrimSpace(name) == "" {
			return Frame{}, fmt.Errorf("column %d: name is blank", i)
		}
		if _, ok := f.index[name]; ok {
			return Frame{}, fmt.Errorf("duplicate column %q", name)
		}
		f.index[name] = i
		f.columns[i] = column{Field{name, Unknown}, make([]any, len(rows))}
	}
	for r, row := range rows {
		if len(row) != len(headers) {
			return Frame{}, fmt.Errorf("row %d: got %d cells, want %d", r, len(row), len(headers))
		}
		for c, v := range row {
			value, kind, err := normalize(v)
			if err != nil {
				return Frame{}, fmt.Errorf("row %d, column %q: %w", r, headers[c], err)
			}
			col := &f.columns[c]
			if kind != Unknown {
				if col.field.Kind != Unknown && col.field.Kind != kind {
					return Frame{}, fmt.Errorf("row %d, column %q: mixed %s and %s values", r, headers[c], col.field.Kind, kind)
				}
				col.field.Kind = kind
			}
			col.values[r] = value
		}
	}
	return f, nil
}

func normalize(v any) (any, Kind, error) {
	var n float64
	switch x := v.(type) {
	case nil:
		return nil, Unknown, nil
	case string:
		return x, String, nil
	case bool:
		return x, Bool, nil
	case time.Time:
		return x, Time, nil
	case float64:
		n = x
	case float32:
		n = float64(x)
	case int:
		if int64(x) > 1<<53 || int64(x) < -(1<<53) {
			return nil, Unknown, fmt.Errorf("integer exceeds exact float64 range")
		}
		n = float64(x)
	case int64:
		if x > 1<<53 || x < -(1<<53) {
			return nil, Unknown, fmt.Errorf("integer exceeds exact float64 range")
		}
		n = float64(x)
	case int32:
		n = float64(x)
	case int16:
		n = float64(x)
	case int8:
		n = float64(x)
	case uint:
		if uint64(x) > 1<<53 {
			return nil, Unknown, fmt.Errorf("integer exceeds exact float64 range")
		}
		n = float64(x)
	case uint64:
		if x > 1<<53 {
			return nil, Unknown, fmt.Errorf("integer exceeds exact float64 range")
		}
		n = float64(x)
	case uint32:
		n = float64(x)
	case uint16:
		n = float64(x)
	case uint8:
		n = float64(x)
	default:
		return nil, Unknown, fmt.Errorf("unsupported value type %T", v)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil, Unknown, fmt.Errorf("nonfinite number")
	}
	if n == 0 {
		n = 0
	} // Normalize negative zero for consistent grouping keys.
	return n, Number, nil
}

// Shape returns the number of rows and columns.
func (f Frame) Shape() (int, int) { return f.rows, len(f.columns) }

// Columns returns the column names in display order.
func (f Frame) Columns() []string {
	out := make([]string, len(f.columns))
	for i, c := range f.columns {
		out[i] = c.field.Name
	}
	return out
}

// Schema returns a copy of the column metadata.
func (f Frame) Schema() []Field {
	out := make([]Field, len(f.columns))
	for i, c := range f.columns {
		out[i] = c.field
	}
	return out
}

// Records returns a copy of the row-oriented data.
func (f Frame) Records() [][]any {
	out := make([][]any, f.rows)
	for r := range out {
		out[r] = make([]any, len(f.columns))
		for c := range f.columns {
			out[r][c] = f.columns[c].values[r]
		}
	}
	return out
}

// At retrieves a cell, distinguishing a missing column/index from a null value.
func (f Frame) At(row int, name string) (any, error) {
	c, ok := f.index[name]
	if !ok {
		return nil, fmt.Errorf("unknown column %q", name)
	}
	if row < 0 || row >= f.rows {
		return nil, fmt.Errorf("row %d out of bounds [0,%d)", row, f.rows)
	}
	return f.columns[c].values[row], nil
}

// Head returns up to n leading rows. Negative n returns no rows.
func (f Frame) Head(n int) Frame {
	if n < 0 {
		n = 0
	}
	if n > f.rows {
		n = f.rows
	}
	out := f
	out.rows = n
	out.columns = append([]column(nil), f.columns...)
	for i := range out.columns {
		out.columns[i].values = out.columns[i].values[:n]
	}
	return out
}

// Select returns named columns in the requested order, sharing immutable data.
// Selecting no columns preserves the number of rows.
func (f Frame) Select(names ...string) (Frame, error) {
	out := Frame{rows: f.rows, index: make(map[string]int, len(names)), columns: make([]column, len(names))}
	for i, name := range names {
		c, ok := f.index[name]
		if !ok {
			return Frame{}, fmt.Errorf("unknown column %q", name)
		}
		if _, ok := out.index[name]; ok {
			return Frame{}, fmt.Errorf("duplicate selection %q", name)
		}
		out.index[name] = i
		out.columns[i] = f.columns[c]
	}
	return out, nil
}

func (f Frame) take(indices []int) Frame {
	out := Frame{rows: len(indices), index: f.index, columns: make([]column, len(f.columns))}
	for i, c := range f.columns {
		values := make([]any, len(indices))
		for j, r := range indices {
			values[j] = c.values[r]
		}
		out.columns[i] = column{c.field, values}
	}
	return out
}

// Row is a read-only view of one row, supplied to a column calculation.
type Row struct {
	frame Frame
	index int
}

// Get returns a cell and whether the named column exists. A null cell is nil,true.
func (r Row) Get(name string) (any, bool) {
	c, ok := r.frame.index[name]
	if !ok {
		return nil, false
	}
	return r.frame.columns[c].values[r.index], true
}

// WithColumn computes or replaces a column without modifying f. Calculation
// errors are wrapped with the row index and can be inspected with errors.Is.
func (f Frame) WithColumn(name string, fn func(Row) (any, error)) (Frame, error) {
	if strings.TrimSpace(name) == "" {
		return Frame{}, fmt.Errorf("column name is blank")
	}
	if fn == nil {
		return Frame{}, fmt.Errorf("column calculation is nil")
	}
	values := make([]any, f.rows)
	kind := Unknown
	for r := range values {
		v, err := fn(Row{f, r})
		if err != nil {
			return Frame{}, fmt.Errorf("row %d: %w", r, err)
		}
		v, k, err := normalize(v)
		if err != nil {
			return Frame{}, fmt.Errorf("row %d: %w", r, err)
		}
		if k != Unknown {
			if kind != Unknown && kind != k {
				return Frame{}, fmt.Errorf("column %q has mixed types", name)
			}
			kind = k
		}
		values[r] = v
	}
	out := Frame{rows: f.rows, columns: append([]column(nil), f.columns...), index: make(map[string]int, len(f.index)+1)}
	for k, v := range f.index {
		out.index[k] = v
	}
	c, ok := out.index[name]
	if !ok {
		c = len(out.columns)
		out.index[name] = c
		out.columns = append(out.columns, column{})
	}
	out.columns[c] = column{Field{name, kind}, values}
	return out, nil
}
