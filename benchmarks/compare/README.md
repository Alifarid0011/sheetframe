# Eager DataFrame comparison

This standalone module compares Sheetframe v0.1.0, the current local Sheetframe source, pandas (default string dtype and an explicit object-string variant), Polars and QFrame. It adds no dependencies to the core Go module. Results concern one synthetic, null-free workload; they do not establish general performance parity or superiority. Excel I/O, nullable/mixed types, joins, frame construction, memory use and developer ergonomics are outside these timings.

[Measured tables and raw evidence](RESULTS.md) are included for the recorded machine; read the scope before interpreting them.

## Reproduce

Use Go 1.25 or newer and Python 3.12 (the measured run used Go 1.26.8 and Python 3.12.14). Create the isolated environment and install the exact tested package versions:

```sh
cd benchmarks/compare
python3.12 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
```

Prepare a clean baseline checkout outside the current repository. In the examples below, replace `/absolute/path/to/sheetframe-v010` with that checkout:

```sh
git clone --branch v0.1.0 --depth 1 https://github.com/Alifarid0011/sheetframe.git /absolute/path/to/sheetframe-v010
.venv/bin/python run.py --baseline /absolute/path/to/sheetframe-v010 --prepare
.venv/bin/python run.py --baseline /absolute/path/to/sheetframe-v010 --reuse-builds --check --rows 2000 --output results/correctness
.venv/bin/python run.py --baseline /absolute/path/to/sheetframe-v010 --reuse-builds --cpu 0 --rows 100000,1000000 --warmups 2 --repeats 7 --output results/single-thread
```

The runner executes engines sequentially, and builds all Go executables before starting any measurements. Run on an otherwise idle machine. Keep the baseline and current sources unchanged after `--prepare`; `--reuse-builds` rejects stale source or harness fingerprints. Without that flag, binaries are rebuilt first. `--check` executes and validates operations without recording timings. `--engines current,qframe,pandas,pandas-object,polars` omits the baseline if only a current comparison is needed.

Default comparable mode sets `GOMAXPROCS`, `POLARS_MAX_THREADS`, `OMP_NUM_THREADS`, `OPENBLAS_NUM_THREADS`, `MKL_NUM_THREADS`, and `NUMEXPR_NUM_THREADS` to 1 before process startup. Optional `--mode default` removes those overrides for a separately labeled run; do not mix its results with single-thread results. The measured single-thread run additionally pins all child processes to CPU 0, a P-core logical CPU on the tested hybrid i3-1215U, with `--cpu 0`. Choose an allowed CPU appropriate to your machine, or omit this optional Linux-only flag; actual affinity is recorded. Leave affinity unrestricted for a default multicore run. Normal garbage collection remains enabled. A forced collection happens before each measured sample, outside its timer, in both Go and Python.

`run.py` uses the ordinary Go module cache and defaults its build cache to `.build/go-cache` if not already configured. Set `GOMODCACHE` or `GOCACHE` normally if desired. `.venv` and `.build` are ignored by Git. Raw result directories are retained, never silently overwritten.

## Dataset and exact operation semantics

For rows `i = 0…n−1`, the same three columns are generated independently before timing:

| Column | Type | Formula |
|---|---|---|
| `group` | string, no category/enum encoding | `g00` … `g19`, from `i % 20` |
| `amount` | float64 | `float64(i % 1000)` |
| `flag` | bool | `(i // 1000) % 2 == 0` |

The flag changes between repeated amount cycles, making incorrect tie ordering observable. Amounts and sums are integral and exactly representable at the tested sizes. No nulls, dates, CSV parsing, Excel parsing or random generation are included. Data generation and each engine's input-frame construction happen before operation timers. The published row counts are 100,000 and 1,000,000.

| Case | Required result |
|---|---|
| `select` | one-column DataFrame containing `amount`, original row order |
| `filter` | `amount >= 500`, retaining all three columns and source row order |
| `group_sum` | sum `amount` by string `group`, output columns `group,total`, first-seen group order |
| `stable_sort` | all columns, descending `amount`, stable equal-key order |
| `pipeline` | eager filter (`amount >= 500`) → first-seen group sum → descending `total` sort |

All engines use their native eager DataFrame operations. Polars uses eager `DataFrame`, not a lazy query plan. pandas uses `sort=False` grouping and `kind="stable"`; Polars uses `maintain_order=True` for group and sort. Native DataFrame results must exist when the timer stops. No deferred export, iterator, or unevaluated lazy query is timed as completed work.

The pinned clean environment has no optional PyArrow package. The `pandas` case uses pandas 3.1.0's default Python-backed string dtype. The additional `pandas-object` case explicitly sets only the group column to `object`, a common pandas string representation, with identical float64 and bool columns. Both are reported; actual dtypes and string storage are recorded in raw metadata. No categorical/enum encoding is used.

Native ownership strategies differ: Sheetframe/QFrame selection can share columns, pandas 3 uses Copy-on-Write, and Polars uses shared Arrow buffers. A cheap selection measures that engine's native DataFrame selection, not a required deep copy. Filter/sort results may likewise use gathered buffers or indexed views. We do not add cross-language row conversion inside the timer, because that would measure conversion rather than the native operation. Every resulting cell is consumed and validated after timing.

### QFrame compatibility

QFrame v0.4.0 is the latest tagged stable release verified from the Go proxy version list. Its sort implementation uses unstable quicksort. The standalone stable-sort case is **skipped**, never presented as equivalent.

Its native group output follows hash-table order. The `group_sum` case therefore includes an extra native ascending sort by group label **inside** the timer. This restores exact first-seen order for this dataset, whose first group occurrences are `g00` through `g19`. It is a workload-specific compatibility adapter, not a general first-seen ordering implementation. The pipeline's final totals are distinct at the tested sizes; its final native descending sort yields the same output without equal-key ambiguity. The full result checksum must still match the oracle.

## Correctness and measurement

Before timing an operation, an independent formula-based oracle constructs the expected shape, column names and SHA-256 digest in source order. Warmups and **every** measured result are checked outside the timer, including all cells and tie order. SHA-256 canonicalization uses UTF-8 TSV rows, integer-formatted numeric values, lowercase `true`/`false` and a trailing newline per row. Native result extraction, checksum work, input setup, process startup and compilation are excluded. The runner also asserts cross-engine hashes agree.

Each operation receives two untimed warmups and seven timed samples by default; the reported median uses those samples. Raw nanoseconds are kept. The harness does not tune repetitions by engine speed or drop outliers. Processes run in a documented fixed order, so thermal drift and other host activity remain possible; rerun on an idle machine before relying on small differences. No statistical significance claim is made from a single run.

Outputs in each result directory:

- `metadata.json`: CPU/platform/thread settings, installed Python versions, source hashes/revisions, harness hash, engine order and host load before/after.
- `{baseline,current,qframe,pandas,pandas-object,polars}.json`: per-engine runtime/dependency details, every sample, medians, correctness digests, exclusions/adapters.
- `summary.csv`: one row per case, including skipped cases and notes.
- `samples.csv`: one row per measured sample.

Go dependencies are pinned in this standalone `go.mod`/`go.sum`; Python packages are pinned in `requirements.txt`. Baseline/current source fingerprints are recorded so a release label cannot conceal local changes. Any published conclusions should cite the raw result directory, machine and thread mode, and include slower as well as faster cases.

## Primary API references

- [pandas stable sort](https://pandas.pydata.org/docs/reference/api/pandas.DataFrame.sort_values.html) and [group ordering](https://pandas.pydata.org/docs/reference/api/pandas.DataFrame.groupby.html).
- [Polars eager sort](https://docs.pola.rs/api/python/stable/reference/dataframe/api/polars.DataFrame.sort.html) and [group_by](https://docs.pola.rs/api/python/stable/reference/dataframe/api/polars.DataFrame.group_by.html).
- [QFrame API](https://pkg.go.dev/github.com/tobgu/qframe@v0.4.0), [sort source](https://github.com/tobgu/qframe/blob/v0.4.0/internal/sort/sorter.go), [group source](https://github.com/tobgu/qframe/blob/v0.4.0/internal/grouper/grouper.go).
