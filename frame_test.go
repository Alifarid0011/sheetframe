package sheetframe

import (
	"bytes"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) Frame {
	t.Helper()
	f, err := New([]string{"team", "amount", "active"}, [][]any{
		{"A", 10, true}, {"B", 30, false}, {"A", 20, true}, {"B", nil, true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestNewValidation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers []string
		rows    [][]any
	}{
		{"duplicate", []string{"x", "x"}, nil},
		{"blank", []string{" "}, nil},
		{"width", []string{"x"}, [][]any{{1, 2}}},
		{"mixed", []string{"x"}, [][]any{{1}, {"one"}}},
		{"nan", []string{"x"}, [][]any{{math.NaN()}}},
		{"inf", []string{"x"}, [][]any{{math.Inf(1)}}},
		{"large integer", []string{"x"}, [][]any{{int64(1<<53 + 1)}}},
		{"unsupported", []string{"x"}, [][]any{{[]int{1}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.headers, tt.rows); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestRejectInvalidUTF8(t *testing.T) {
	for _, value := range []string{"\xff", "\xfe"} {
		if _, err := New([]string{"key"}, [][]any{{value}}); err == nil {
			t.Fatal("invalid UTF-8 accepted as group/join key")
		}
		if _, err := New([]string{value}, nil); err == nil {
			t.Fatal("invalid UTF-8 header accepted")
		}
	}
	f := fixture(t)
	if _, err := f.WithColumn("key", func(Row) (any, error) { return "\xff", nil }); err == nil {
		t.Fatal("calculation bypassed string validation")
	}
	if _, err := ReadCSV(strings.NewReader("key\n\xff\n")); err == nil {
		t.Fatal("CSV bypassed string validation")
	}
}

func TestFrameImmutability(t *testing.T) {
	headers := []string{"x"}
	rows := [][]any{{1}, {nil}}
	f, err := New(headers, rows)
	if err != nil {
		t.Fatal(err)
	}
	headers[0] = "changed"
	rows[0][0] = 99
	got, err := f.At(0, "x")
	if err != nil || got != float64(1) {
		t.Fatalf("At = %v, %v", got, err)
	}
	copyRows := f.Records()
	copyRows[0][0] = 88
	cols := f.Columns()
	cols[0] = "changed"
	schema := f.Schema()
	schema[0].Name = "changed"
	if got, _ := f.At(0, "x"); got != float64(1) {
		t.Fatal("backing data exposed")
	}
	if !reflect.DeepEqual(f.Schema(), []Field{{"x", Number}}) {
		t.Fatalf("schema %v", f.Schema())
	}
	if _, err := f.At(-1, "x"); err == nil {
		t.Fatal("negative index accepted")
	}
	if _, err := f.At(0, "missing"); err == nil {
		t.Fatal("missing column accepted")
	}
}

func TestSelectAndHead(t *testing.T) {
	f := fixture(t)
	s, err := f.Select("amount", "team")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Columns(), []string{"amount", "team"}) {
		t.Fatal(s.Columns())
	}
	if _, err := f.Select("team", "team"); err == nil {
		t.Fatal("duplicate selection accepted")
	}
	if _, err := f.Select("missing"); err == nil {
		t.Fatal("missing selection accepted")
	}
	s, err = f.Select()
	if err != nil {
		t.Fatal(err)
	}
	if r, c := s.Shape(); r != 4 || c != 0 {
		t.Fatalf("shape %d %d", r, c)
	}
	for _, n := range []int{-1, 0, 2, 99} {
		r, _ := f.Head(n).Shape()
		want := n
		if want < 0 {
			want = 0
		}
		if want > 4 {
			want = 4
		}
		if r != want {
			t.Fatalf("Head(%d)=%d", n, r)
		}
	}
}

func TestFilter(t *testing.T) {
	f := fixture(t)
	for _, tt := range []struct {
		op    Op
		value any
		count int
	}{
		{Eq, 20, 1}, {Ne, 20, 2}, {Gt, 10, 2}, {Ge, 20, 2}, {Lt, 20, 1}, {Le, 20, 2}, {IsNull, nil, 1}, {NotNull, nil, 3},
	} {
		got, err := f.Filter("amount", tt.op, tt.value)
		if err != nil {
			t.Fatal(err)
		}
		r, _ := got.Shape()
		if r != tt.count {
			t.Errorf("%s: %d want %d", tt.op, r, tt.count)
		}
	}
	for _, tt := range []struct {
		col string
		op  Op
		v   any
	}{{"missing", Eq, 1}, {"amount", Eq, "x"}, {"amount", Contains, "x"}, {"team", Op("bad"), "A"}, {"amount", Eq, nil}} {
		if _, err := f.Filter(tt.col, tt.op, tt.v); err == nil {
			t.Fatalf("accepted invalid filter %+v", tt)
		}
	}
	s, err := f.Filter("team", Contains, "A")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := s.Shape()
	if r != 2 {
		t.Fatal(r)
	}
}

func TestSort(t *testing.T) {
	f := fixture(t)
	for _, desc := range []bool{false, true} {
		s, err := f.Sort("amount", desc)
		if err != nil {
			t.Fatal(err)
		}
		want := [][]any{{"A", float64(10), true}, {"A", float64(20), true}, {"B", float64(30), false}, {"B", nil, true}}
		if desc {
			want[0], want[2] = want[2], want[0]
		}
		if !reflect.DeepEqual(s.Records(), want) {
			t.Fatalf("sort=%v", s.Records())
		}
	}
	s, err := f.Sort("team", false)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.At(1, "amount"); got != float64(20) {
		t.Fatal("sort is not stable")
	}
	if _, err := f.Sort("missing", false); err == nil {
		t.Fatal("missing column accepted")
	}
}

func TestWithColumn(t *testing.T) {
	f := fixture(t)
	g, err := f.WithColumn("double", func(r Row) (any, error) {
		v, ok := r.Get("amount")
		if !ok {
			t.Fatal("column absent")
		}
		if v == nil {
			return nil, nil
		}
		return v.(float64) * 2, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := g.At(2, "double"); v != float64(40) {
		t.Fatal(v)
	}
	if _, c := f.Shape(); c != 3 {
		t.Fatal("original changed")
	}
	sentinel := errors.New("calculation failed")
	if _, err := f.WithColumn("x", func(Row) (any, error) { return nil, sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

func TestGroupBy(t *testing.T) {
	g, err := fixture(t).GroupBy("team")
	if err != nil {
		t.Fatal(err)
	}
	f, err := g.Aggregate(Aggregation{"amount", Sum, "total"}, Aggregation{"amount", Mean, "average"}, Aggregation{"amount", Count, "n"}, Aggregation{"amount", Min, "low"}, Aggregation{"amount", Max, "high"})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]any{{"A", float64(30), float64(15), float64(2), float64(10), float64(20)}, {"B", float64(30), float64(30), float64(1), float64(30), float64(30)}}
	if !reflect.DeepEqual(f.Records(), want) {
		t.Fatalf("aggregate=%v", f.Records())
	}
	nulls, _ := New([]string{"key", "v"}, [][]any{{nil, nil}, {"x", nil}})
	groups, _ := nulls.GroupBy("key")
	out, err := groups.Aggregate(Aggregation{"v", Mean, "avg"}, Aggregation{"v", Sum, "sum"}, Aggregation{"v", Count, "n"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Records(), [][]any{{nil, nil, float64(0), float64(0)}, {"x", nil, float64(0), float64(0)}}) {
		t.Fatal(out.Records())
	}
	if _, err := fixture(t).GroupBy(); err == nil {
		t.Fatal("empty keys accepted")
	}
	if _, err := g.Aggregate(Aggregation{"team", Sum, "bad"}); err == nil {
		t.Fatal("string sum accepted")
	}
	if _, err := g.Aggregate(Aggregation{"amount", Sum, "team"}); err == nil {
		t.Fatal("name collision accepted")
	}
}

func TestJoin(t *testing.T) {
	left, _ := New([]string{"id", "name"}, [][]any{{1, "a"}, {2, "b"}, {nil, "c"}})
	right, _ := New([]string{"id", "name"}, [][]any{{1, "x"}, {1, "y"}, {nil, "z"}})
	out, err := left.Join(right, []string{"id"}, Left)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]any{{float64(1), "a", "x"}, {float64(1), "a", "y"}, {float64(2), "b", nil}, {nil, "c", nil}}
	if !reflect.DeepEqual(out.Records(), want) {
		t.Fatalf("join=%v", out.Records())
	}
	if !reflect.DeepEqual(out.Columns(), []string{"id", "name", "name_right"}) {
		t.Fatal(out.Columns())
	}
	out, err = left.Join(right, []string{"id"}, Inner)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := out.Shape(); n != 2 {
		t.Fatal(n)
	}
	if _, err = left.Join(right, []string{"missing"}, Inner); err == nil {
		t.Fatal("missing join key accepted")
	}
}

func TestCSV(t *testing.T) {
	f, err := ReadCSV(strings.NewReader("name,amount,active\nalpha,12,true\nbeta,,false\n"))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := f.WriteCSV(&out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "name,amount,active\nalpha,12,true\nbeta,,false\n" {
		t.Fatal(out.String())
	}
	if _, err := ReadCSV(strings.NewReader("x,x\n1,2\n")); err == nil {
		t.Fatal("duplicate header accepted")
	}
}

func TestTimeAndEmpty(t *testing.T) {
	tm := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	f, err := New([]string{"t"}, [][]any{{tm}, {nil}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.Sort("t", true)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s.At(0, "t"); v != tm {
		t.Fatal(v)
	}
	empty, err := f.Filter("t", Gt, tm)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(empty.Schema(), f.Schema()) {
		t.Fatal("empty selection lost schema")
	}
}
