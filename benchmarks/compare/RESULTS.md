# Measured comparison — 28 October 2026

These are local measurements of native eager frame operations on one deterministic null-free dataset, not a general ranking of DataFrame libraries. All input construction, process startup, checksumming and export are outside operation timers. Both pandas string representations are included. No overall winner or pandas parity is claimed.

- CPU: 12th Gen Intel Core i3-1215U; Linux x86_64; all single-thread processes pinned to logical CPU 0, a P-core thread.
- Threads: GOMAXPROCS, POLARS_MAX_THREADS, OMP, OpenBLAS, MKL and NumExpr limits all 1. Normal GC enabled, with explicit collection outside each sample.
- Versions: Go 1.26.8, Python 3.12.14, pandas 3.1.0, NumPy 2.3.5, Polars/runtime-32 2.1.0, QFrame 0.4.0. Optional PyArrow is absent.
- Each case: two untimed warmups, seven measured samples, median below in milliseconds. Every measured output fully validated after timing. All 58 compatible cases and 406 measured samples pass; QFrame's two standalone stable-sort cases are excluded.
- Current core commit: `d43d108a68f715a59fed85a7c823a04a44d62308`; includes group and numeric-sort optimizations. Baseline is the source snapshot of v0.1.0. Exact source hashes and local replace paths are in raw metadata.

## 100,000 input rows

| Operation | Sheetframe v0.1.0 | Sheetframe current | QFrame | pandas default string | pandas object string | Polars |
|---|---:|---:|---:|---:|---:|---:|
| Select amount | 0.005 | 0.009 | 0.004 | 1.032 | 0.967 | 0.334 |
| Filter amount ≥ 500 | 2.117 | 2.193 | 0.669 | 2.694 | 2.669 | 0.977 |
| First-seen group sum | 70.767 | 5.071 | 17.464 | 17.329 | 10.263 | 4.128 |
| Stable descending sort | 56.735 | 21.813 | Excluded | 8.078 | 8.466 | 14.266 |
| Filter → group → sort | 47.277 | 4.720 | 8.813 | 12.170 | 8.845 | 3.636 |

## 1,000,000 input rows

| Operation | Sheetframe v0.1.0 | Sheetframe current | QFrame | pandas default string | pandas object string | Polars |
|---|---:|---:|---:|---:|---:|---:|
| Select amount | 0.011 | 0.010 | 0.011 | 0.946 | 0.934 | 0.336 |
| Filter amount ≥ 500 | 24.932 | 24.684 | 5.650 | 18.790 | 18.496 | 7.182 |
| First-seen group sum | 694.083 | 51.658 | 177.515 | 162.788 | 83.385 | 28.455 |
| Stable descending sort | 714.268 | 281.514 | Excluded | 112.855 | 119.290 | 200.294 |
| Filter → group → sort | 455.073 | 50.306 | 91.934 | 94.959 | 59.646 | 19.656 |

## Read the scope with the numbers

The updated core reduces group, pipeline and numeric-sort time relative to the release baseline on this dataset. It does not improve every case. Filtering is largely unchanged, and filtering and stable sorting still take longer than several peers here. pandas' object-string grouping is materially faster than its Python-backed default string dtype, so both results matter. Polars remains ahead on grouping and the pipeline in this single-thread workload.

Native selection is particularly sensitive to ownership: the Go frames can share existing columns, pandas 3 uses Copy-on-Write, and Polars shares Arrow buffers. These are native DataFrame selections, not mandatory deep copies; do not use the selection row to infer overall throughput. Absolute selection timings are small and sensitive to wrapper/timer overhead.

QFrame's unordered group result is sorted by label inside the group timer to reproduce first-seen g00…g19 order for this specific dataset. Its unstable quicksort cannot implement the standalone stable-sort contract, so that case is excluded. Pipeline totals are distinct at both tested sizes, removing equal-key ambiguity in its final sort.

The input amount is an integral-valued float64 in 0…999, with 20 repeating string keys. This is not representative of every numeric distribution or cardinality. Nulls, mixed types, joins, Excel/CSV I/O, construction cost and memory consumption are outside this comparison. The numeric sort optimization uses additional temporary memory; latency results do not establish lower memory usage. Fixed engine order, host background activity and thermal drift can affect small differences; no statistical significance is asserted from this one run.

## Raw evidence and reproduction

- [Full measurement setup and API references](README.md).
- [Single-thread metadata](results/single-thread-final-20261028/metadata.json), [summary CSV](results/single-thread-final-20261028/summary.csv), [all raw samples](results/single-thread-final-20261028/samples.csv).
- Per-engine JSON: [baseline](results/single-thread-final-20261028/baseline.json), [current](results/single-thread-final-20261028/current.json), [QFrame](results/single-thread-final-20261028/qframe.json), [pandas default](results/single-thread-final-20261028/pandas.json), [pandas object](results/single-thread-final-20261028/pandas-object.json), [Polars](results/single-thread-final-20261028/polars.json).

Earlier pilot, correctness-only and interrupted pre-final measurements were retained outside the public result directory for audit; they are not inputs to these tables.

## Separate Polars default-thread run

This additional run removes all thread overrides and CPU pinning. It is a different configuration, not part of the single-thread table above. Polars reported an 8-thread pool, with affinity to CPUs 0…7, on the same hybrid i3-1215U. Two warmups and seven samples were used; every result validated. Medians in milliseconds:

| Operation | 100,000 rows | 1,000,000 rows |
|---|---:|---:|
| Select amount | 0.322 | 0.350 |
| Filter amount ≥ 500 | 1.476 | 6.217 |
| First-seen group sum | 4.837 | 10.813 |
| Stable descending sort | 4.607 | 61.152 |
| Filter → group → sort | 5.874 | 13.452 |

The default-thread run changes both parallelism and scheduling affinity; differences cannot be attributed solely to parallelism. It must not be substituted into the single-thread comparison. See [metadata](results/polars-default-threads-20261028/metadata.json), [raw engine JSON](results/polars-default-threads-20261028/polars.json), [summary](results/polars-default-threads-20261028/summary.csv) and [all samples](results/polars-default-threads-20261028/samples.csv).
