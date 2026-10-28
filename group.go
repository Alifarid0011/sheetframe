package sheetframe

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// AggOp describes an aggregate calculation over non-null values.
type AggOp string

const (
	Count AggOp = "count"
	Sum   AggOp = "sum"
	Mean  AggOp = "mean"
	Min   AggOp = "min"
	Max   AggOp = "max"
)

// Aggregation names the source column, calculation, and output column.
type Aggregation struct {
	Column string `json:"column"`
	Op     AggOp  `json:"op"`
	As     string `json:"as"`
}

// Grouped is a frame with grouping keys, ready to aggregate.
type Grouped struct {
	frame Frame
	keys  []int
}

// GroupBy groups one or more columns. Nulls form groups like any other key.
func (f Frame) GroupBy(names ...string) (Grouped, error) {
	if len(names) == 0 {
		return Grouped{}, fmt.Errorf("at least one group key is required")
	}
	keys, err := f.keyColumns(names)
	if err != nil {
		return Grouped{}, err
	}
	return Grouped{f, keys}, nil
}

func (f Frame) keyColumns(names []string) ([]int, error) {
	keys := make([]int, len(names))
	seen := map[string]bool{}
	for i, n := range names {
		c, ok := f.index[n]
		if !ok {
			return nil, fmt.Errorf("unknown column %q", n)
		}
		if seen[n] {
			return nil, fmt.Errorf("duplicate key %q", n)
		}
		seen[n] = true
		keys[i] = c
	}
	return keys, nil
}

// rowKey uses a JSON array to preserve boundaries, quoting, and nulls. Times
// are normalized to UTC so equal instants in different zones share a key.
func (f Frame) rowKey(r int, keys []int) (string, []any, bool, error) {
	values := make([]any, len(keys))
	hasNull := false
	for i, c := range keys {
		v := f.columns[c].values[r]
		if v == nil {
			hasNull = true
		}
		if tm, ok := v.(time.Time); ok {
			v = tm.UTC()
		}
		values[i] = v
	}
	b, err := json.Marshal(values)
	return string(b), values, hasNull, err
}

// Aggregate returns one row per group, in first-seen order. Count counts
// non-null values; an all-null Sum is zero, while Mean/Min/Max are null.
func (g Grouped) Aggregate(aggs ...Aggregation) (Frame, error) {
	if len(g.keys) == 0 {
		return Frame{}, fmt.Errorf("no group keys")
	}
	if len(aggs) == 0 {
		return Frame{}, fmt.Errorf("at least one aggregation is required")
	}
	headers := make([]string, 0, len(g.keys)+len(aggs))
	seen := map[string]bool{}
	for _, c := range g.keys {
		n := g.frame.columns[c].field.Name
		headers = append(headers, n)
		seen[n] = true
	}
	cols := make([]int, len(aggs))
	for i, a := range aggs {
		c, ok := g.frame.index[a.Column]
		if !ok {
			return Frame{}, fmt.Errorf("unknown column %q", a.Column)
		}
		cols[i] = c
		if seen[a.As] || a.As == "" {
			return Frame{}, fmt.Errorf("aggregation output %q must be nonblank and unique", a.As)
		}
		seen[a.As] = true
		headers = append(headers, a.As)
		k := g.frame.columns[c].field.Kind
		switch a.Op {
		case Count:
		case Sum, Mean:
			if k != Unknown && k != Number {
				return Frame{}, fmt.Errorf("%s requires numeric column %q", a.Op, a.Column)
			}
		case Min, Max:
			if k == Bool {
				return Frame{}, fmt.Errorf("%s does not support bool column %q", a.Op, a.Column)
			}
		default:
			return Frame{}, fmt.Errorf("unknown aggregation %q", a.Op)
		}
	}
	groups := [][]int{}
	keys := [][]any{}
	lookup := map[string]int{}
	for r := 0; r < g.frame.rows; r++ {
		key, values, _, err := g.frame.rowKey(r, g.keys)
		if err != nil {
			return Frame{}, fmt.Errorf("group key: %w", err)
		}
		i, ok := lookup[key]
		if !ok {
			i = len(groups)
			lookup[key] = i
			groups = append(groups, nil)
			keys = append(keys, values)
		}
		groups[i] = append(groups[i], r)
	}
	rows := make([][]any, len(groups))
	for i, members := range groups {
		row := append([]any(nil), keys[i]...)
		for j, a := range aggs {
			count := 0
			sum, comp := 0.0, 0.0
			var extreme any
			for _, r := range members {
				v := g.frame.columns[cols[j]].values[r]
				if v == nil {
					continue
				}
				count++
				switch a.Op {
				case Sum, Mean:
					n := v.(float64)
					y := n - comp
					t := sum + y
					comp = (t - sum) - y
					sum = t
				case Min:
					if extreme == nil || compare(v, extreme) < 0 {
						extreme = v
					}
				case Max:
					if extreme == nil || compare(v, extreme) > 0 {
						extreme = v
					}
				}
			}
			var result any
			switch a.Op {
			case Count:
				result = float64(count)
			case Sum:
				result = sum
			case Mean:
				if count > 0 {
					result = sum / float64(count)
				}
			case Min, Max:
				result = extreme
			}
			if n, ok := result.(float64); ok && (math.IsInf(n, 0) || math.IsNaN(n)) {
				return Frame{}, fmt.Errorf("aggregation %q overflowed", a.As)
			}
			row = append(row, result)
		}
		rows[i] = row
	}
	out, err := New(headers, rows)
	if err != nil {
		return Frame{}, err
	}
	// Keep useful types even when no rows or only null aggregate values remain.
	for i, c := range g.keys {
		out.columns[i].field.Kind = g.frame.columns[c].field.Kind
	}
	for j, a := range aggs {
		k := g.frame.columns[cols[j]].field.Kind
		if a.Op == Count || a.Op == Sum || a.Op == Mean {
			k = Number
		}
		out.columns[len(g.keys)+j].field.Kind = k
	}
	return out, nil
}
