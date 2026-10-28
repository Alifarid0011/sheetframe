package sheetframe

import (
	"math"
	"math/rand/v2"
	"sort"
	"testing"
)

// Compare with the standard stable sorter across the entire finite float64
// domain, not just the dense integer-like numbers used in the comparison data.
func TestNumericSortWideRange(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(41, 73))
	rows := make([][]any, 4096)
	for i := range rows {
		var value any
		switch {
		case i%17 == 0:
		case i%7 == 0:
			value = float64(i%11 - 5)
		default:
			for {
				n := math.Float64frombits(rng.Uint64())
				if !math.IsNaN(n) && !math.IsInf(n, 0) {
					value = n
					break
				}
			}
		}
		rows[i] = []any{value, i}
	}
	f, err := New([]string{"key", "id"}, rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, desc := range []bool{false, true} {
		order := make([]int, len(rows))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			a, b := rows[order[i]][0], rows[order[j]][0]
			if a == nil {
				return false
			}
			if b == nil {
				return true
			}
			if desc {
				return a.(float64) > b.(float64)
			}
			return a.(float64) < b.(float64)
		})
		out, err := f.Sort("key", desc)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range order {
			if got, _ := out.At(i, "id"); got != float64(want) {
				t.Fatalf("descending=%v row%d id=%v want%d", desc, i, got, want)
			}
		}
	}
}

func TestNumericSortAllNullAndEqual(t *testing.T) {
	t.Parallel()
	for _, nulls := range []bool{false, true} {
		rows := make([][]any, 2049)
		for i := range rows {
			var value any = 2
			if nulls && i < len(rows)-1 {
				value = nil
			}
			rows[i] = []any{value, i}
		}
		f, err := New([]string{"key", "id"}, rows)
		if err != nil {
			t.Fatal(err)
		}
		if nulls {
			f, err = f.Filter("key", IsNull, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, desc := range []bool{false, true} {
			out, err := f.Sort("key", desc)
			if err != nil {
				t.Fatal(err)
			}
			count, _ := f.Shape()
			for i := 0; i < count; i++ {
				if id, _ := out.At(i, "id"); id != float64(i) {
					t.Fatalf("nulls=%v descending=%v row%d id=%v", nulls, desc, i, id)
				}
			}
		}
	}
}
