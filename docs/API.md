# API guide

Import the core as `sf "github.com/Alifarid0011/sheetframe"` and Excel I/O as
`"github.com/Alifarid0011/sheetframe/xlsx"`.

## Fluent pipelines

Use `Query` to compose operations and check one error at the end:

```go
totals, err := sales.Query().
    Filter("revenue", sf.Ge, 1000).
    GroupBy("region").
    Aggregate(sf.Aggregation{Column: "revenue", Op: sf.Sum, As: "total"}).
    Sort("total", true).
    Collect()
if err != nil { return err }
```

Operations execute immediately. `Collect` retrieves the result and first error;
it does not trigger a lazy optimizer. Later operations and callbacks are skipped
after an error. Query values can be reused to create independent branches without
mutating the input. The individual `Frame` methods remain available when you want
to handle errors at each step. See the [executable example](../example_query_test.go).

## Values and ownership

`New([]string, [][]any)` validates a rectangular table and copies its input.
Columns must have unique nonblank names and homogeneous non-null types. Text
values and names must be valid UTF-8; invalid sequences return errors. Integer
and float inputs normalize to `float64`; supported other types are `string`,
`bool`, and `time.Time`. `nil` is a missing cell. No mutable maps, slices, pointers,
or arbitrary objects are accepted as cells. Large integer inputs outside ±2^53,
NaN, infinity, and mixed-type columns produce errors.

A frame's zero value is an empty table. `Columns`, `Schema`, and `Records` return
copies. `At` reports missing columns and row bounds as errors. `Select` and `Head`
can share private immutable data. Other operations build new columns. Reading a
frame concurrently is safe; callbacks must synchronize any external shared state.

## Filter and sort

```go
high, err := sales.Filter("revenue", sf.Ge, 1000)
known, err := sales.Filter("revenue", sf.NotNull, nil)
text, err := sales.Filter("region", sf.Contains, "North")
sorted, err := sales.Sort("revenue", true) // descending; nulls still last
```

Operators: `Eq`, `Ne`, `Lt`, `Le`, `Gt`, `Ge`, `Contains`, `IsNull`, `NotNull`.
Ordinary comparisons exclude nulls, including `Ne`. Use explicit null operators;
`Eq` with a nil comparison is an error. Comparison types must match; numeric
arguments normalize like frame inputs. `Contains` is case-sensitive. Time
comparison uses instants; string ordering is bytewise lexicographic. Sorting is
stable, with nulls last in both directions. Chain filters to combine conditions.

## Calculated columns

```go
result, err := sales.WithColumn("net", func(row sf.Row) (any, error) {
    value, exists := row.Get("revenue")
    if !exists { return nil, fmt.Errorf("missing revenue") }
    if value == nil { return nil, nil }
    return value.(float64) * 0.8, nil
})
```

`WithColumn` appends a column or replaces an existing column in its original
position. Callback errors include the row index and retain `errors.Is` semantics.
The resulting column is validated. Empty calculations have `Unknown` kind because
there are no observed values from which to infer a type.

## Aggregation

```go
groups, err := sales.GroupBy("region", "product")
summary, err := groups.Aggregate(
    sf.Aggregation{Column: "revenue", Op: sf.Sum, As: "total"},
    sf.Aggregation{Column: "revenue", Op: sf.Mean, As: "average"},
    sf.Aggregation{Column: "revenue", Op: sf.Count, As: "observed"},
)
```

Group order follows the first appearance of each key. Null keys form groups;
compound keys preserve value boundaries. Times are normalized to UTC when
building group/join keys, so equal instants in different zones match.

`Count` counts **non-null cells**, not all group rows. `Sum` and `Mean` require
numeric columns; `Min`/`Max` accept numbers, strings, and times. All-null `Sum` and
`Count` are zero; all-null `Mean`/`Min`/`Max` are nil. Summation uses compensation;
nonfinite results return errors. Group keys and output names must be unique,
nonblank, and not collide. Aggregation types survive an empty result.

## Joins

```go
joined, err := sales.Join(regions, []string{"region_id"}, sf.Left)
```

`Inner` emits matches; `Left` also emits unmatched left rows with null right cells.
Duplicate keys produce every matching pair, in left-row then right-match order.
Join key types must match. Null keys **never match**, including other null keys.
The right copy of each key is omitted. Other duplicate names receive `_right`;
remaining name collisions produce an error. A many-to-many join can expand the
result substantially; size inputs accordingly.

## CSV

`ReadCSV(io.Reader)` treats the first row as headers. Nonblank cells are inferred
per column: numbers first, then literal `true`/`false`, then strings. Blanks are
null; dates are strings until explicitly converted with `WithColumn`. Numeric
inference can remove leading zeros and uses float64 precision; use XLSX typed
strings for identifier columns. CSV cannot distinguish an empty string from null
on roundtrip. `WriteCSV(io.Writer)` writes standard quoted CSV and propagates
writer errors. Strings are written verbatim, including formula-like text.

## Excel

```go
frame, err := xlsx.Read(reader, xlsx.Options{Sheet: "Sales", HeaderRow: 2})
err = xlsx.Write(writer, frame, "Report")
```

Zero-value read options select the first sheet and header row 1. A blank export
sheet name selects `Sheet1`. Caller readers/writers are never closed by this
package. Header validation, unexpected extra data columns, malformed cells,
unsupported mixed types, and I/O errors are surfaced. Missing trailing cells are
null. Source row indices in DataFrame validation messages are zero-based relative
to the first data row.

Stored Excel strings remain strings, even when they look numeric. Built-in date
styles 14–22/45–47 and ISO date cells are recognized; custom date formats remain
numeric serials. Both Excel date systems are supported on import. Dates use UTC;
Export accepts UTC years 1900–9999 and rejects longer strings/headers than
32,767 UTF-16 units; it never silently truncates them. Excel subsecond precision
still applies. Boolean and numeric cell types survive
export/import. A header-only worksheet has `Unknown` kinds because no values
provide type information.

Only **cached formula values** are read. The adapter does not calculate formulas.
Excel error cells return errors. Export is a fresh values workbook, not a
workbook editing operation: formulas, original styles, charts, and macros are
not preserved. Text is exported as literal XLSX text, not executable formulas.

## Costs and current scope

Select shares columns and costs O(selected columns). Filter/copy costs O(rows ×
columns); sort uses a stable index sort plus materialization. Hash grouping and
joins encode compound keys as JSON arrays, favoring clear collision-free semantics
over custom key encodings; this entails per-row allocations. All execution is
eager and in memory. [Benchmarks](BENCHMARKS.md) show reproducible measured costs.
