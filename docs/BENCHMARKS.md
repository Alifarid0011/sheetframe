# Benchmarks

[Readable live comparison](https://alifarid0011.github.io/sheetframe/benchmarks.html) · [Raw samples](../benchmarks/compare/results/single-thread-final-20261028) · [Reproduction instructions](../benchmarks/compare/README.md)

## Cross-library comparison

Measured 2026-10-28 on Linux/amd64, Intel Core i3-1215U, Go 1.26.8 and Python 3.12.14. Each engine has one thread, pinned to the same P-core logical CPU (CPU 0). Two warmups, seven samples per operation; tables report median milliseconds. Input construction, imports, startup and result validation are excluded. Ordinary garbage collection is enabled; explicit collections occur before samples outside the timer. Every result is fully checked against independent row/order SHA-256 oracles.

Dataset: three columns, 20 repeated string groups, float64 amounts `i % 1000`, boolean flags. No nulls, dates or categoricals. This is one synthetic workload, not general performance parity or a pandas API comparison. Excel I/O, joins, frame creation and browser/WASM execution are not measured here.

Versions: pandas 3.1.0 (both default Python-backed strings and explicit object strings), Polars 2.1.0, QFrame 0.4.0; optional PyArrow is absent. Baseline is Sheetframe v0.1.0 (`134328b`); current core is `d43d108` (the v0.2.0 implementation). Raw source hashes identify the actual code: Go replacement-module dependency fields retain the nominal requested `v0.1.0` version while the runner replaces the source directory for each build. The current core SHA-256 is `2123c2347c8630182ba4ec661134565d23548f1f7f74f757bad3fb5d88d856bb`.

### 100,000 rows — median ms

| Operation | Sheetframe 0.1 | Sheetframe 0.2 | pandas default | pandas object | Polars | QFrame |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Select one column | 0.005 | 0.009 | 1.032 | 0.967 | 0.334 | 0.004 |
| Filter | 2.117 | 2.193 | 2.694 | 2.669 | 0.977 | 0.669 |
| Group + sum | 70.767 | 5.071 | 17.329 | 10.263 | 4.128 | 17.464 |
| Stable numeric sort | 56.735 | 21.813 | 8.078 | 8.466 | 14.266 | omitted¹ |
| Filter → group → sort | 47.277 | 4.720 | 12.170 | 8.845 | 3.636 | 8.813 |

### 1,000,000 rows — median ms

| Operation | Sheetframe 0.1 | Sheetframe 0.2 | pandas default | pandas object | Polars | QFrame |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Select one column | 0.011 | 0.010 | 0.946 | 0.934 | 0.336 | 0.011 |
| Filter | 24.932 | 24.684 | 18.790 | 18.496 | 7.182 | 5.650 |
| Group + sum | 694.083 | 51.658 | 162.788 | 83.385 | 28.455 | 177.515 |
| Stable numeric sort | 714.268 | 281.514 | 112.855 | 119.290 | 200.294 | omitted¹ |
| Filter → group → sort | 455.073 | 50.306 | 94.959 | 59.646 | 19.656 | 91.934 |

¹ QFrame uses an unstable sort; it is omitted from the stable-sort case. Its group case includes a native sort **inside** the timer to restore first-seen `g00…g19` group order for this dataset. Pipeline totals are distinct, so its final native sort yields the required order. See the [compatibility details](../benchmarks/compare/README.md#qframe-compatibility).

## Interpretation

At one million rows, the current core is about **13.4× faster at group/sum**, **2.5× faster at stable numeric sort**, and **9.0× faster at the filter/group/sort pipeline** than v0.1.0 in this run. Group/sum is about **1.6× faster than pandas object strings**, **3.2× faster than pandas default strings**, and **3.4× faster than QFrame** for the specified workload. Polars is faster at group/sum and the pipeline. pandas is faster at stable sorting; QFrame is faster at filtering. Sheetframe is not universally faster than these tools.

Selection can share private immutable data. Other engines use their native sharing, Copy-on-Write, gathered buffers or indexed views. Selection timings are not deep-copy timings and should not be interpreted as bulk scanning throughput. Cross-language conversion and row export are excluded from operation timing, then fully consumed during validation.

All measurements run in a fixed documented engine order. Host activity, cache effects and thermal drift remain possible even with affinity; raw samples and load metadata are included. No small difference is claimed statistically significant. Higher cardinality, other types, distributions and thread counts can change rankings.

## Separate Polars default-thread run

A separately recorded Polars run used its default eight-thread pool and unrestricted CPU affinity. At one million rows: group/sum **10.813 ms**, stable sort **61.152 ms**, filter **6.217 ms**, pipeline **13.452 ms**. These are different execution settings and are not mixed into the single-thread table. [Full separate results and caveats](../benchmarks/compare/RESULTS.md#separate-polars-default-thread-run).

## Native allocation and distribution checks

The core also supplies `BenchmarkFrame` (100,000 rows, three columns, 20 groups) and `BenchmarkNumericSort` (100,000 rows, one column, wide random finite float64 values, nulls, sorted inputs and equal values). Setup is excluded:

```sh
go test -run '^$' -bench '^BenchmarkFrame$' -benchmem -count=5 -benchtime=200ms
go test -run '^$' -bench '^BenchmarkNumericSort$' -benchmem -count=3 -benchtime=200ms
```

Native results are a separate experiment from the cross-language harness. Numeric radix sorting uses extra temporary key/index buffers: up to **16 bytes per row** beyond the prior index buffer. It preserves stable equal-key ordering, nulls last in both directions, and all finite float64 values. Ordered inputs skip scattering; arbitrary values need at most eight byte passes.

Final `BenchmarkFrame` allocation run: Go 1.26.8, `GOMAXPROCS=1`, pinned CPU 0, five samples of 200ms each. Setup excluded; no competing benchmark process. The separate cross-engine table uses its own dataset and sampling harness.

| Version | Operation (100k rows) | Median ms | Bytes/op | Allocs/op |
| --- | --- | ---: | ---: | ---: |
| v0.1.0 | Select100k | 0.0007 | 320 | 3 |
| v0.1.0 | Filter100k | 6.9523 | 3,211,448 | 6 |
| v0.1.0 | GroupSum100k | 78.5302 | 9,772,861 | 400,444 |
| v0.1.0 | Sort100k | 67.7643 | 5,619,944 | 7 |
| v0.2.0 | Select100k | 0.0007 | 320 | 3 |
| v0.2.0 | Filter100k | 6.8835 | 3,211,448 | 6 |
| v0.2.0 | GroupSum100k | 5.5060 | 8,992 | 162 |
| v0.2.0 | Sort100k | 18.1739 | 7,225,520 | 7 |


Additional distribution checks (Go 1.26.8, `GOMAXPROCS=2`, no affinity, three samples, median ms) compare the same one-column workload on baseline/current code:

| Numeric distribution | v0.1.0 | Current |
| --- | ---: | ---: |
| Wide random finite floats | 124.299 | 15.710 |
| Wide floats with nulls | 117.585 | 18.207 |
| Already sorted | 4.878 | 3.607 |
| All equal | 4.996 | 3.876 |

These distribution measurements are environment-specific, not cross-library comparisons. [Raw native output](../benchmarks/native) includes every sample, allocated bytes/op and allocations/op. Per-operation allocations do not measure total resident memory or cross-language memory use.

## Reproduce and inspect

The isolated [comparison harness](../benchmarks/compare) pins Go and Python dependencies without adding competitors to the library's core dependencies. Its [README](../benchmarks/compare/README.md) documents installation, CPU/thread control, eager operation semantics, source fingerprints, forced-GC boundaries, correctness checks and all exclusions. Re-render the public table from a completed, validated run:

```sh
python3 scripts/render-benchmarks.py benchmarks/compare/results/single-thread-final-20261028
```

Browser demo timings measure actual WebAssembly work on the visitor's device, using different data and execution conditions; they cannot be compared directly with these native results.
