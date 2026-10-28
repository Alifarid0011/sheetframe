package sheetframe

import (
	"fmt"
	"testing"
)

func benchmarkFrame(b *testing.B, n int) Frame {
	b.Helper()
	rows := make([][]any, n)
	for i := range rows {
		rows[i] = []any{fmt.Sprintf("team-%d", i%20), float64(i % 1000), i%2 == 0}
	}
	f, err := New([]string{"team", "amount", "active"}, rows)
	if err != nil {
		b.Fatal(err)
	}
	return f
}

// Benchmarks measure eager in-memory operations; they exclude input creation
// and XLSX parsing. Reproduce with go test -run '^$' -bench . -benchmem.
func BenchmarkFrame(b *testing.B) {
	f := benchmarkFrame(b, 100000)
	b.Run("Select100k", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := f.Select("amount"); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Filter100k", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := f.Filter("amount", Ge, 500); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("GroupSum100k", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			g, _ := f.GroupBy("team")
			if _, err := g.Aggregate(Aggregation{"amount", Sum, "total"}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Sort100k", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := f.Sort("amount", true); err != nil {
				b.Fatal(err)
			}
		}
	})
}
