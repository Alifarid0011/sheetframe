# sheetframe

**Excel tables. Go pipelines.**

A focused, pandas-inspired DataFrame library for Go. Load a worksheet, filter and
summarize its rows, join another table, and write the result back to Excel.

Built by [alifarid](https://github.com/Alifarid0011).

[Interactive playground](https://alifarid0011.github.io/sheetframe/) ·
[API guide](docs/API.md) · [Examples](example_test.go) · [Benchmarks](docs/BENCHMARKS.md)

## Install

Requires **Go 1.25+**.

```sh
go get github.com/Alifarid0011/sheetframe
```

The core package uses only the Go standard library. Import the optional `xlsx`
package for Excel I/O; it uses [Excelize](https://github.com/qax-os/excelize).
The original DataFrame implementation is independent of Excelize.

## Excel → analysis → Excel

```go
package main

import (
    "log"
    "os"

    sf "github.com/Alifarid0011/sheetframe"
    "github.com/Alifarid0011/sheetframe/xlsx"
)

func main() {
    input, err := os.Open("sales.xlsx")
    if err != nil { log.Fatal(err) }
    defer input.Close()

    sales, err := xlsx.Read(input, xlsx.Options{Sheet: "Sales"})
    if err != nil { log.Fatal(err) }
    totals, err := sales.Query().
        Filter("revenue", sf.Ge, 1000).
        GroupBy("region").
        Aggregate(sf.Aggregation{
            Column: "revenue", Op: sf.Sum, As: "total_revenue",
        }).
        Sort("total_revenue", true).
        Collect()
    if err != nil { log.Fatal(err) }

    output, err := os.Create("totals.xlsx")
    if err != nil { log.Fatal(err) }
    if err := xlsx.Write(output, totals, "Totals"); err != nil {
        output.Close()
        log.Fatal(err)
    }
    if err := output.Close(); err != nil { log.Fatal(err) }
}
```

## What you can do

| Task | API |
| --- | --- |
| Build a typed table | `New(headers, rows)` |
| Chain operations with one error check | `Query().Filter(...).GroupBy(...).Aggregate(...).Sort(...).Collect()` |
| Inspect data | `Shape`, `Schema`, `Columns`, `At`, `Records`, `Head` |
| Project columns | `Select` |
| Filter rows | `Filter` with comparisons, `Contains`, `IsNull`, `NotNull` |
| Stable sort | `Sort`, with nulls last in either direction |
| Calculate or replace a column | `WithColumn` |
| Group and summarize | `GroupBy(...).Aggregate(...)`: count, sum, mean, min, max |
| Combine tables | `Join`: inner and left hash joins, including duplicate keys |
| CSV I/O | `ReadCSV`, `WriteCSV` |
| Excel I/O | `xlsx.Read`, `xlsx.Write` |

Operations leave their inputs unchanged. Columns store homogeneous, nullable
values. `Select` shares private immutable data; row-changing operations allocate
new columns. Returned slices never expose the frame's backing storage.

## A small Go-native API

```go
frame, err := sf.New(
    []string{"region", "revenue"},
    [][]any{{"North", 1200}, {"South", 900}, {"North", nil}},
)
if err != nil { return err }

withTax, err := frame.WithColumn("tax", func(row sf.Row) (any, error) {
    value, _ := row.Get("revenue")
    if value == nil { return nil, nil }
    return value.(float64) * 0.2, nil
})
```

See the [API guide](docs/API.md) for join examples, null behavior, and errors.

## Honest boundaries

This is an early, eager, in-memory library, **not a pandas replacement**. It does
not implement Series arithmetic, indexing/MultiIndex, windows, pivot tables,
plotting, out-of-core processing, or the complete pandas API. There is no claim
that it outperforms pandas or other Go DataFrame libraries.

- Text values and column names must be valid UTF-8. Invalid text is rejected before
  it can create ambiguous group or join keys.
- Values are `string`, `float64`, `bool`, `time.Time`, or `nil`. Numeric inputs are
  normalized to float64. Nonfinite values, mixed column types, and integer inputs
  outside ±2^53 are rejected. Use strings for large identifiers or exact decimals.
- XLSX reading preserves typed strings, including leading zeros. CSV inference
  can interpret numeric-looking identifiers as numbers; blanks become null.
- Excel formulas use their saved cached values. There is no formula calculation
  engine. Export creates a new workbook of values: original formulas, styles,
  charts, and macros are not preserved.
- Built-in Excel date formats 14–22 and 45–47 and ISO date cells are recognized.
  Custom date number formats remain numeric serials. Excel dates are interpreted
  in UTC. Export rejects dates outside UTC years 1900–9999 and text/headers over
  32,767 UTF-16 units instead of silently changing or truncating data. Excel
  subsecond precision constraints apply.
- Text is written literally to XLSX. CSV writes values verbatim; a spreadsheet
  application may interpret formula-like CSV cells when opening a CSV file.
- Input tables and results must fit in memory. Many-to-many joins can expand
  dramatically. This version has no streaming DataFrame execution engine.

[Excelize](https://github.com/qax-os/excelize) handles workbook compatibility;
[QFrame](https://github.com/tobgu/qframe) and
[Gota](https://github.com/go-gota/gota) are related Go DataFrame projects worth
considering for other needs. Sheetframe focuses on a small Excel-to-analysis path.

## Try the real engine in your browser

The [playground](https://alifarid0011.github.io/sheetframe/) compiles this Go module
to WebAssembly. Upload CSV/XLSX, filter, group, sort, and download the full result.
Files stay in your browser. The table previews at most 200 rows; exports include
all result rows. Displayed timings are measured in your browser and are not native
Go benchmarks. Initial WebAssembly loading is larger than an ordinary static page.

Run it locally:

```sh
bash scripts/build-demo.sh
python3 -m http.server 8000 --directory docs
# open http://localhost:8000
```

## Development

```sh
go test ./...
go vet ./...
go test -race ./...
go test -run '^$' -bench BenchmarkFrame -benchmem
```

[Contributing](CONTRIBUTING.md) · [MIT license](LICENSE)
