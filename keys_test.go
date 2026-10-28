package sheetframe

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// These cases catch ambiguous compound encodings, UTC normalization mistakes,
// and accidental differences between the single-key and compound-key paths.
func TestGroupAndJoinKeySemantics(t *testing.T) {
	t.Parallel()
	instant := time.Date(2026, 10, 28, 12, 0, 0, 123, time.UTC)
	for _, tc := range []struct {
		name string
		keys []string
		rows [][]any
		want [][]any
	}{
		{"boundaries", []string{"a", "b"}, [][]any{{"a", "bc", 1}, {"ab", "c", 2}, {"a", "bc", 3}}, [][]any{{"a", "bc", float64(4)}, {"ab", "c", float64(2)}}},
		{"null and empty", []string{"a", "b"}, [][]any{{nil, "", 1}, {"", nil, 2}, {nil, "", 3}, {"", "", 4}}, [][]any{{nil, "", float64(4)}, {"", nil, float64(2)}, {"", "", float64(4)}}},
		{"embedded delimiters", []string{"a", "b"}, [][]any{{"a\x00\"", "b,c", 1}, {"a", "\x00\"b,c", 2}}, [][]any{{"a\x00\"", "b,c", float64(1)}, {"a", "\x00\"b,c", float64(2)}}},
		{"signed zero", []string{"a"}, [][]any{{math.Copysign(0, -1), 1}, {0, 2}}, [][]any{{float64(0), float64(3)}}},
		{"bool", []string{"a"}, [][]any{{false, 1}, {true, 2}, {false, 3}}, [][]any{{false, float64(4)}, {true, float64(2)}}},
		{"time zones", []string{"a"}, [][]any{{instant, 1}, {instant.In(time.FixedZone("offset", 3600)), 2}}, [][]any{{instant, float64(3)}}},
		{"compound time", []string{"a", "b"}, [][]any{{instant, "x", 1}, {instant.In(time.FixedZone("offset", -7200)), "x", 2}}, [][]any{{instant, "x", float64(3)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			headers := append(append([]string(nil), tc.keys...), "value")
			f, err := New(headers, tc.rows)
			if err != nil {
				t.Fatal(err)
			}
			g, err := f.GroupBy(tc.keys...)
			if err != nil {
				t.Fatal(err)
			}
			out, err := g.Aggregate(Aggregation{"value", Sum, "total"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(out.Records(), tc.want) {
				t.Fatalf("group=%v want=%v", out.Records(), tc.want)
			}
			joined, err := f.Join(out, tc.keys, Left)
			if err != nil {
				t.Fatal(err)
			}
			for r, row := range tc.rows {
				hasNull := false
				for c := range tc.keys {
					hasNull = hasNull || row[c] == nil
				}
				v, err := joined.At(r, "total")
				if err != nil {
					t.Fatal(err)
				}
				if hasNull && v != nil {
					t.Fatalf("null key matched: %v", joined.Records())
				}
				if !hasNull && v == nil {
					t.Fatalf("non-null key lost: %v", joined.Records())
				}
			}
		})
	}
}

// time.Time is an accepted core value even outside JSON's four-digit years.
// Grouping and joining must not inherit an unrelated JSON encoding restriction.
func TestTimeKeysOutsideJSONRange(t *testing.T) {
	t.Parallel()
	for _, year := range []int{-1, 10000} {
		for _, compound := range []bool{false, true} {
			tm := time.Date(year, 1, 2, 3, 4, 5, 6, time.UTC)
			headers, keys := []string{"when", "value"}, []string{"when"}
			rows := [][]any{{tm, 1}, {tm.In(time.FixedZone("offset", 3600)), 2}}
			if compound {
				headers, keys = []string{"when", "label", "value"}, []string{"when", "label"}
				rows = [][]any{{tm, "x", 1}, {tm.In(time.FixedZone("offset", 3600)), "x", 2}}
			}
			f, err := New(headers, rows)
			if err != nil {
				t.Fatal(err)
			}
			g, err := f.GroupBy(keys...)
			if err != nil {
				t.Fatal(err)
			}
			out, err := g.Aggregate(Aggregation{"value", Sum, "total"})
			if err != nil {
				t.Fatalf("year %d compound %v: %v", year, compound, err)
			}
			if r, _ := out.Shape(); r != 1 {
				t.Fatalf("equal instants split into %d groups", r)
			}
			if total, _ := out.At(0, "total"); total != float64(3) {
				t.Fatal(total)
			}
			joined, err := f.Join(out, keys, Inner)
			if err != nil {
				t.Fatal(err)
			}
			if r, _ := joined.Shape(); r != 2 {
				t.Fatalf("join returned %d rows", r)
			}
		}
	}
}

func TestAggregateOverflow(t *testing.T) {
	t.Parallel()
	f, err := New([]string{"key", "value"}, [][]any{{"x", math.MaxFloat64}, {"x", math.MaxFloat64}})
	if err != nil {
		t.Fatal(err)
	}
	g, err := f.GroupBy("key")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []AggOp{Sum, Mean} {
		if _, err := g.Aggregate(Aggregation{"value", op, "result"}); err == nil {
			t.Fatalf("%s accepted overflow", op)
		}
	}
}

func TestSortStableTiesAndNulls(t *testing.T) {
	t.Parallel()
	tm := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, values := range [][]any{
		{2, nil, 1, 2, nil, 1},
		{"b", nil, "a", "b", nil, "a"},
		{true, nil, false, true, nil, false},
		{tm.Add(time.Hour), nil, tm, tm.Add(time.Hour), nil, tm},
	} {
		rows := make([][]any, len(values))
		for i, v := range values {
			rows[i] = []any{v, i}
		}
		f, err := New([]string{"key", "id"}, rows)
		if err != nil {
			t.Fatal(err)
		}
		for _, desc := range []bool{false, true} {
			out, err := f.Sort("key", desc)
			if err != nil {
				t.Fatal(err)
			}
			want := []float64{2, 5, 0, 3, 1, 4}
			if desc {
				want = []float64{0, 3, 2, 5, 1, 4}
			}
			for i, id := range want {
				if got, _ := out.At(i, "id"); got != id {
					t.Fatalf("%T descending=%v row%d=%v want%v", values[0], desc, i, got, id)
				}
			}
		}
	}
}
