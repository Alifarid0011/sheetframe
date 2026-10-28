package sheetframe

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Op is a filter operation. Use IsNull/NotNull explicitly for missing values.
type Op string

const (
	Eq       Op = "eq"
	Ne       Op = "ne"
	Lt       Op = "lt"
	Le       Op = "le"
	Gt       Op = "gt"
	Ge       Op = "ge"
	Contains Op = "contains"
	IsNull   Op = "is_null"
	NotNull  Op = "not_null"
)

// Filter selects matching rows. Ordinary comparisons never match nulls.
// Comparisons require a value of the column's type; Contains requires a string.
func (f Frame) Filter(name string, op Op, value any) (Frame, error) {
	c, ok := f.index[name]
	if !ok {
		return Frame{}, fmt.Errorf("unknown column %q", name)
	}
	switch op {
	case Eq, Ne, Lt, Le, Gt, Ge, Contains, IsNull, NotNull:
	default:
		return Frame{}, fmt.Errorf("unknown filter operation %q", op)
	}
	col := f.columns[c]
	if op != IsNull && op != NotNull {
		v, k, err := normalize(value)
		if err != nil {
			return Frame{}, err
		}
		if k == Unknown {
			return Frame{}, fmt.Errorf("use IsNull or NotNull for null comparisons")
		}
		if col.field.Kind != Unknown && col.field.Kind != k {
			return Frame{}, fmt.Errorf("column %q is %s, comparison is %s", name, col.field.Kind, k)
		}
		if op == Contains && k != String {
			return Frame{}, fmt.Errorf("Contains requires strings")
		}
		value = v
	}
	indices := make([]int, 0, f.rows)
	for r, v := range col.values {
		match := false
		switch op {
		case IsNull:
			match = v == nil
		case NotNull:
			match = v != nil
		case Contains:
			if v != nil {
				match = strings.Contains(v.(string), value.(string))
			}
		default:
			if v != nil {
				cmp := compare(v, value)
				switch op {
				case Eq:
					match = cmp == 0
				case Ne:
					match = cmp != 0
				case Lt:
					match = cmp < 0
				case Le:
					match = cmp <= 0
				case Gt:
					match = cmp > 0
				case Ge:
					match = cmp >= 0
				}
			}
		}
		if match {
			indices = append(indices, r)
		}
	}
	return f.take(indices), nil
}

// Sort stably sorts a column, placing nulls last in either direction.
func (f Frame) Sort(name string, descending bool) (Frame, error) {
	c, ok := f.index[name]
	if !ok {
		return Frame{}, fmt.Errorf("unknown column %q", name)
	}
	indices := make([]int, f.rows)
	for i := range indices {
		indices[i] = i
	}
	values := f.columns[c].values
	if f.rows >= 2048 && f.columns[c].field.Kind == Number {
		return f.take(sortNumbers(values, indices, descending)), nil
	}
	// Original row positions break ties, preserving stable order while using
	// the faster in-place sorter without an additional value buffer.
	slices.SortFunc(indices, func(i, j int) int {
		a, b := values[i], values[j]
		if a == nil {
			if b == nil {
				return cmp.Compare(i, j)
			}
			return 1
		}
		if b == nil {
			return -1
		}
		order := compare(a, b)
		if order == 0 {
			return cmp.Compare(i, j)
		}
		if descending {
			return -order
		}
		return order
	})
	return f.take(indices), nil
}

func compare(a, b any) int {
	switch x := a.(type) {
	case float64:
		y := b.(float64)
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	case string:
		return strings.Compare(x, b.(string))
	case bool:
		y := b.(bool)
		if !x && y {
			return -1
		}
		if x && !y {
			return 1
		}
	case time.Time:
		y := b.(time.Time)
		if x.Before(y) {
			return -1
		}
		if x.After(y) {
			return 1
		}
	}
	return 0
}
