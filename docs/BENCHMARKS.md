# Benchmarks

Measured on linux/amd64, Intel Core i3-1215U, Go 1.26.8, using 100,000 rows and three columns (20 team groups, numeric amounts, boolean flags). Input construction is excluded; operations are eager. Three runs, shown below in full. These numbers are environment-specific, not a comparison with pandas or another library.

```sh
go test -run '^$' -bench '^BenchmarkFrame$' -benchmem -count=3
```

Approximate medians:

| Operation | Time/op | Allocated bytes/op | Allocs/op |
| --- | ---: | ---: | ---: |
| Select one column | 691 ns | 320 | 3 |
| Filter amount >= 500 | 6.46 ms | 3,211,449 | 6 |
| Group by team, sum | 97.8 ms | 9,774,298 | ~400,449 |
| Stable sort by amount | 93.3 ms | 5,619,944 | 7 |

Selection shares immutable backing columns; it does not scan 100,000 values. Filtering and sorting allocate result columns. Grouping currently favors straightforward JSON compound keys, with significant per-row allocations; this is a visible optimization opportunity, not a zero-allocation claim. These benchmarks exclude XLSX import/export. Browser demo timings use WebAssembly and different datasets, so they cannot be compared directly.

## Raw output

```text
goos: linux
goarch: amd64
pkg: github.com/Alifarid0011/sheetframe
cpu: 12th Gen Intel(R) Core(TM) i3-1215U
BenchmarkFrame/Select100k-8         	 1672198	       691.4 ns/op	     320 B/op	       3 allocs/op
BenchmarkFrame/Select100k-8         	 1792959	       664.0 ns/op	     320 B/op	       3 allocs/op
BenchmarkFrame/Select100k-8         	 1554172	       832.7 ns/op	     320 B/op	       3 allocs/op
BenchmarkFrame/Filter100k-8         	     199	   6606274 ns/op	 3211450 B/op	       6 allocs/op
BenchmarkFrame/Filter100k-8         	     195	   5413614 ns/op	 3211449 B/op	       6 allocs/op
BenchmarkFrame/Filter100k-8         	     180	   6461087 ns/op	 3211449 B/op	       6 allocs/op
BenchmarkFrame/GroupSum100k-8       	      12	  94443526 ns/op	 9774298 B/op	  400449 allocs/op
BenchmarkFrame/GroupSum100k-8       	      14	  97780421 ns/op	 9774207 B/op	  400448 allocs/op
BenchmarkFrame/GroupSum100k-8       	      12	 108609257 ns/op	 9774312 B/op	  400450 allocs/op
BenchmarkFrame/Sort100k-8           	      10	 127200702 ns/op	 5619944 B/op	       7 allocs/op
BenchmarkFrame/Sort100k-8           	      12	  93307741 ns/op	 5619944 B/op	       7 allocs/op
BenchmarkFrame/Sort100k-8           	      12	  85841602 ns/op	 5619944 B/op	       7 allocs/op
PASS
ok  	github.com/Alifarid0011/sheetframe	24.899s
```
