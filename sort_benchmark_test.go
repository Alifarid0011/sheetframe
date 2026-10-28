package sheetframe

import (
	"math"
	"math/rand/v2"
	"testing"
)

// Wider distributions keep dense integer-like comparison data from being the
// only workload used to assess numeric sorting. Setup is excluded from timing.
func BenchmarkNumericSort(b *testing.B) {
	for _, name := range []string{"wide", "sorted", "equal", "nullable"} {
		b.Run(name, func(b *testing.B) {
			rng := rand.New(rand.NewPCG(41, 73))
			rows := make([][]any, 100000)
			for i := range rows {
				var value any = float64(i)
				switch name {
				case "wide", "nullable":
					for {
						n := math.Float64frombits(rng.Uint64())
						if !math.IsNaN(n) && !math.IsInf(n, 0) {
							value = n
							break
						}
					}
					if name == "nullable" && i%17 == 0 {
						value = nil
					}
				case "equal":
					value = float64(1)
				}
				rows[i] = []any{value}
			}
			f, err := New([]string{"value"}, rows)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := f.Sort("value", false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
