package sheetframe

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestQueryPipeline(t *testing.T) {
	f, err := New([]string{"team", "amount"}, [][]any{{"A", 600}, {"B", 900}, {"A", 700}, {"B", 100}})
	if err != nil {
		t.Fatal(err)
	}
	before := f.Records()
	got, err := f.Query().Filter("amount", Ge, 500).GroupBy("team").Aggregate(Aggregation{"amount", Sum, "total"}).Sort("total", true).Collect()
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]any{{"A", float64(1300)}, {"B", float64(900)}}; !reflect.DeepEqual(got.Records(), want) {
		t.Fatalf("got %v, want %v", got.Records(), want)
	}
	if !reflect.DeepEqual(f.Records(), before) {
		t.Fatal("query changed its source frame")
	}
}

func TestQueryBranches(t *testing.T) {
	f, err := New([]string{"amount"}, [][]any{{1}, {2}, {3}})
	if err != nil {
		t.Fatal(err)
	}
	base := f.Query().Filter("amount", Ge, 2)
	first, err := base.Head(1).Collect()
	if err != nil {
		t.Fatal(err)
	}
	last, err := base.Sort("amount", true).Head(1).Collect()
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := base.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Records(), [][]any{{float64(2)}}) || !reflect.DeepEqual(last.Records(), [][]any{{float64(3)}}) {
		t.Fatal(first.Records(), last.Records())
	}
	if n, _ := unchanged.Shape(); n != 2 {
		t.Fatalf("base query changed: %v", unchanged.Records())
	}
}

func TestQueryFirstErrorSkipsLaterCallbacks(t *testing.T) {
	f, err := New([]string{"amount"}, [][]any{{1}})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("calculation failed")
	calls := 0
	failed := f.Query().WithColumn("bad", func(Row) (any, error) { return nil, sentinel })
	got, err := failed.Filter("missing", Eq, 1).Select("missing").Sort("missing", false).Head(1).WithColumn("later", func(Row) (any, error) { calls++; return 2, nil }).Join(f, nil, Inner).GroupBy("missing").Aggregate(Aggregation{"missing", Sum, "bad"}).Collect()
	if !errors.Is(err, sentinel) {
		t.Fatalf("first error lost: %v", err)
	}
	if calls != 0 {
		t.Fatalf("callback called %d times after failure", calls)
	}
	if r, c := got.Shape(); r != 0 || c != 0 {
		t.Fatalf("failed Collect returned %dx%d frame", r, c)
	}
}

func TestQueryOperationErrors(t *testing.T) {
	f, err := New([]string{"key", "amount"}, [][]any{{"a", 1}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name  string
		query Query
		want  string
	}{
		{"select", f.Query().Select("missing"), "unknown column"},
		{"filter", f.Query().Filter("amount", Eq, "wrong"), "comparison"},
		{"sort", f.Query().Sort("missing", false), "unknown column"},
		{"group", f.Query().GroupBy("missing").Aggregate(Aggregation{"amount", Sum, "total"}), "unknown column"},
		{"aggregate", f.Query().GroupBy("key").Aggregate(Aggregation{"key", Sum, "total"}), "numeric"},
		{"empty aggregate", f.Query().GroupBy("key").Aggregate(), "at least one aggregation"},
		{"join", f.Query().Join(f, []string{"missing"}, Left), "unknown column"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.query.Collect(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestQuerySelectWithColumnAndJoin(t *testing.T) {
	f, err := New([]string{"id", "amount"}, [][]any{{1, 2}, {2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	lookup, err := New([]string{"id", "label"}, [][]any{{1, "first"}, {2, "second"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Query().WithColumn("double", func(row Row) (any, error) { v, _ := row.Get("amount"); return v.(float64) * 2, nil }).Join(lookup, []string{"id"}, Left).Select("label", "double").Collect()
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]any{{"first", float64(4)}, {"second", float64(6)}}; !reflect.DeepEqual(got.Records(), want) {
		t.Fatalf("got %v, want %v", got.Records(), want)
	}
}

func TestQueryZeroValues(t *testing.T) {
	var q Query
	got, err := q.Head(1).Select().Collect()
	if err != nil {
		t.Fatal(err)
	}
	if r, c := got.Shape(); r != 0 || c != 0 {
		t.Fatalf("zero query returned %dx%d", r, c)
	}
	var grouped GroupedQuery
	if _, err := grouped.Aggregate(Aggregation{"v", Sum, "total"}).Collect(); err == nil {
		t.Fatal("zero grouped query accepted aggregation")
	}
}

func TestGroupedQueryBranches(t *testing.T) {
	f, err := New([]string{"key", "amount"}, [][]any{{"a", 2}, {"a", 3}})
	if err != nil {
		t.Fatal(err)
	}
	grouped := f.Query().GroupBy("key")
	if _, err := grouped.Aggregate(Aggregation{"missing", Sum, "total"}).Collect(); err == nil {
		t.Fatal("missing aggregate column accepted")
	}
	sum, err := grouped.Aggregate(Aggregation{"amount", Sum, "total"}).Collect()
	if err != nil {
		t.Fatal(err)
	}
	count, err := grouped.Aggregate(Aggregation{"amount", Count, "count"}).Collect()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sum.Records(), [][]any{{"a", float64(5)}}) || !reflect.DeepEqual(count.Records(), [][]any{{"a", float64(2)}}) {
		t.Fatal(sum.Records(), count.Records())
	}
}
