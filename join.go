package sheetframe

import "fmt"

// JoinKind selects matched rows only (Inner), or every left row (Left).
type JoinKind string

const (
	Inner JoinKind = "inner"
	Left  JoinKind = "left"
)

// Join performs a hash join, preserving left-row order and right-match order.
// Duplicate keys produce every matching pair. Null keys never match. Overlapping
// right column names receive a _right suffix; unresolved collisions are errors.
func (f Frame) Join(right Frame, on []string, how JoinKind) (Frame, error) {
	if how != Inner && how != Left {
		return Frame{}, fmt.Errorf("unsupported join kind %q", how)
	}
	if len(on) == 0 {
		return Frame{}, fmt.Errorf("at least one join key is required")
	}
	lkeys, err := f.keyColumns(on)
	if err != nil {
		return Frame{}, err
	}
	rkeys, err := right.keyColumns(on)
	if err != nil {
		return Frame{}, err
	}
	for i, c := range lkeys {
		a, b := f.columns[c].field.Kind, right.columns[rkeys[i]].field.Kind
		if a != Unknown && b != Unknown && a != b {
			return Frame{}, fmt.Errorf("join key %q has incompatible types", on[i])
		}
	}
	headers := f.Columns()
	seen := map[string]bool{}
	for _, n := range headers {
		seen[n] = true
	}
	skip := map[int]bool{}
	for _, c := range rkeys {
		skip[c] = true
	}
	extra := []int{}
	for c, col := range right.columns {
		if skip[c] {
			continue
		}
		name := col.field.Name
		if seen[name] {
			name += "_right"
		}
		if seen[name] {
			return Frame{}, fmt.Errorf("join output column collision %q", name)
		}
		seen[name] = true
		headers = append(headers, name)
		extra = append(extra, c)
	}
	lookup := map[any][]int{}
	rightKeys := rowKeys{frame: right, cols: rkeys}
	for r := 0; r < right.rows; r++ {
		key, null := rightKeys.at(r)
		if !null {
			lookup[key] = append(lookup[key], r)
		}
	}
	rows := [][]any{}
	leftKeys := rowKeys{frame: f, cols: lkeys}
	for r := 0; r < f.rows; r++ {
		key, null := leftKeys.at(r)
		matches := lookup[key]
		if null {
			matches = nil
		}
		if len(matches) == 0 && how == Left {
			matches = []int{-1}
		}
		for _, rr := range matches {
			row := make([]any, len(headers))
			for c, col := range f.columns {
				row[c] = col.values[r]
			}
			if rr >= 0 {
				for i, c := range extra {
					row[len(f.columns)+i] = right.columns[c].values[rr]
				}
			}
			rows = append(rows, row)
		}
	}
	out, err := New(headers, rows)
	if err != nil {
		return Frame{}, err
	}
	for i, c := range f.columns {
		out.columns[i].field.Kind = c.field.Kind
	}
	for i, c := range extra {
		out.columns[len(f.columns)+i].field.Kind = right.columns[c].field.Kind
	}
	return out, nil
}
