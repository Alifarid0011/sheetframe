// Command compare times eager native frame operations; all validation is untimed.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	sf "github.com/Alifarid0011/sheetframe"
	"github.com/tobgu/qframe"
	"github.com/tobgu/qframe/config/groupby"
	"github.com/tobgu/qframe/config/newqf"
	"github.com/tobgu/qframe/types"
)

var operations = []string{"select", "filter", "group_sum", "stable_sort", "pipeline"}
var inputColumns = []string{"group", "amount", "flag"}

type result struct {
	Engine     string   `json:"engine"`
	Version    string   `json:"version"`
	InputRows  int      `json:"input_rows"`
	Operation  string   `json:"operation"`
	SamplesNS  []int64  `json:"samples_ns,omitempty"`
	MedianNS   float64  `json:"median_ns,omitempty"`
	OutputRows int      `json:"output_rows"`
	Columns    []string `json:"columns,omitempty"`
	SHA256     string   `json:"sha256,omitempty"`
	Status     string   `json:"status"`
	Note       string   `json:"note,omitempty"`
}
type report struct {
	Metadata map[string]any `json:"metadata"`
	Results  []result       `json:"results"`
}
type table struct {
	rows    int
	columns []string
	at      func(int, int) any
}

func main() {
	engine := flag.String("engine", "sheetframe", "sheetframe or qframe")
	label := flag.String("label", "sheetframe-current", "engine name in results")
	sizes := flag.String("rows", "100000,1000000", "comma-separated dataset sizes")
	repeats := flag.Int("repeats", 7, "measured samples per operation")
	warmups := flag.Int("warmups", 2, "untimed warmup calls per operation")
	check := flag.Bool("check", false, "correctness checks only; no timings")
	flag.Parse()
	if *engine != "sheetframe" && *engine != "qframe" {
		fail(fmt.Errorf("unknown engine %q", *engine))
	}
	if *repeats < 1 || *warmups < 0 {
		fail(fmt.Errorf("repeats must be positive and warmups nonnegative"))
	}
	info, _ := debug.ReadBuildInfo()
	version := "unknown"
	dependencies := map[string]string{}
	if info != nil {
		for _, d := range info.Deps {
			dependencies[d.Path] = d.Version
			if d.Replace != nil {
				dependencies[d.Path] += " => " + d.Replace.Path
			}
			if (*engine == "sheetframe" && d.Path == "github.com/Alifarid0011/sheetframe") || (*engine == "qframe" && d.Path == "github.com/tobgu/qframe") {
				version = d.Version
			}
		}
	}
	cpu := "unknown"
	if content, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(line, "model name") {
				cpu = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				break
			}
		}
	}
	out := report{Metadata: map[string]any{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpu": cpu, "logical_cpus": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "gogc": os.Getenv("GOGC"), "dependencies": dependencies, "warmups": *warmups, "repeats": *repeats, "check_only": *check, "timing": "native eager operation result; checksum/export/GC before sample excluded; normal GC during operation enabled"}}
	for _, size := range strings.Split(*sizes, ",") {
		n, err := strconv.Atoi(size)
		if err != nil || n < 1000 {
			fail(fmt.Errorf("row count must be integer >=1000: %q", size))
		}
		groups := make([]string, n)
		amounts := make([]float64, n)
		flags := make([]bool, n)
		for i := range groups {
			groups[i] = fmt.Sprintf("g%02d", i%20)
			amounts[i] = float64(i % 1000)
			flags[i] = (i/1000)%2 == 0
		}
		var base any
		if *engine == "sheetframe" {
			rows := make([][]any, n)
			for i := range rows {
				rows[i] = []any{groups[i], amounts[i], flags[i]}
			}
			base, err = sf.New(inputColumns, rows)
			if err != nil {
				fail(err)
			}
		} else {
			q := qframe.New(map[string]types.DataSlice{"group": groups, "amount": amounts, "flag": flags}, newqf.ColumnOrder(inputColumns...))
			if q.Err != nil {
				fail(q.Err)
			}
			base = q
		}
		// Dataset construction and frame loading finish before any timer starts.
		for _, op := range operations {
			r := result{Engine: *label, Version: version, InputRows: n, Operation: op, Status: "ok"}
			if *engine == "qframe" && op == "stable_sort" {
				r.Status = "skipped"
				r.Note = "QFrame v0.4.0 uses unstable quicksort; incompatible equal-key order"
				out.Results = append(out.Results, r)
				continue
			}
			if *engine == "qframe" && op == "group_sum" {
				r.Note = "Timed native group-label sort restores first-seen g00..g19 order for this dataset"
			}
			wantRows, wantColumns, wantHash := expected(n, op)
			validate := func(value any) {
				t := asTable(value)
				if t.rows != wantRows || strings.Join(t.columns, "\x00") != strings.Join(wantColumns, "\x00") {
					fail(fmt.Errorf("%s %s shape mismatch: %d %v expected %d %v", *label, op, t.rows, t.columns, wantRows, wantColumns))
				}
				got := checksum(t)
				if got != wantHash {
					fail(fmt.Errorf("%s %s n=%d checksum %s != %s", *label, op, n, got, wantHash))
				}
				r.OutputRows = t.rows
				r.Columns = t.columns
				r.SHA256 = got
			}
			if *check {
				value, err := operate(base, *engine, op)
				if err != nil {
					fail(err)
				}
				validate(value)
			} else {
				for i := 0; i < *warmups; i++ {
					value, err := operate(base, *engine, op)
					if err != nil {
						fail(err)
					}
					validate(value)
				}
				for i := 0; i < *repeats; i++ {
					runtime.GC()
					start := time.Now()
					value, err := operate(base, *engine, op)
					elapsed := time.Since(start).Nanoseconds()
					if err != nil {
						fail(err)
					}
					r.SamplesNS = append(r.SamplesNS, elapsed)
					validate(value)
					runtime.KeepAlive(value)
				}
				ordered := append([]int64(nil), r.SamplesNS...)
				sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
				mid := len(ordered) / 2
				r.MedianNS = float64(ordered[mid])
				if len(ordered)%2 == 0 {
					r.MedianNS = (float64(ordered[mid-1]) + float64(ordered[mid])) / 2
				}
			}
			out.Results = append(out.Results, r)
		}
		runtime.KeepAlive(base)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		fail(err)
	}
}

func operate(base any, engine, op string) (any, error) {
	if engine == "sheetframe" {
		f := base.(sf.Frame)
		switch op {
		case "select":
			return f.Select("amount")
		case "filter":
			return f.Filter("amount", sf.Ge, 500.0)
		case "stable_sort":
			return f.Sort("amount", true)
		case "group_sum", "pipeline":
			var err error
			if op == "pipeline" {
				f, err = f.Filter("amount", sf.Ge, 500.0)
				if err != nil {
					return nil, err
				}
			}
			g, err := f.GroupBy("group")
			if err != nil {
				return nil, err
			}
			f, err = g.Aggregate(sf.Aggregation{Column: "amount", Op: sf.Sum, As: "total"})
			if err != nil {
				return nil, err
			}
			if op == "pipeline" {
				return f.Sort("total", true)
			}
			return f, nil
		}
	} else {
		f := base.(qframe.QFrame)
		switch op {
		case "select":
			f = f.Select("amount")
		case "filter":
			f = f.Filter(qframe.Filter{Column: "amount", Comparator: ">=", Arg: 500.0})
		case "group_sum", "pipeline":
			if op == "pipeline" {
				f = f.Filter(qframe.Filter{Column: "amount", Comparator: ">=", Arg: 500.0})
			}
			f = f.GroupBy(groupby.Columns("group")).Aggregate(qframe.Aggregation{Column: "amount", Fn: "sum", As: "total"})
			if op == "pipeline" {
				f = f.Sort(qframe.Order{Column: "total", Reverse: true})
			} else {
				f = f.Sort(qframe.Order{Column: "group"})
			}
		}
		return f, f.Err
	}
	return nil, fmt.Errorf("unsupported operation %q", op)
}

func asTable(value any) table {
	if f, ok := value.(sf.Frame); ok {
		rows := f.Records()
		count, _ := f.Shape()
		return table{count, f.Columns(), func(r, c int) any { return rows[r][c] }}
	}
	f := value.(qframe.QFrame)
	names := f.ColumnNames()
	read := make([]func(int) any, len(names))
	for i, name := range names {
		switch name {
		case "group":
			v := f.MustStringView(name)
			read[i] = func(r int) any { return *v.ItemAt(r) }
		case "amount", "total":
			v := f.MustFloatView(name)
			read[i] = func(r int) any { return v.ItemAt(r) }
		case "flag":
			v := f.MustBoolView(name)
			read[i] = func(r int) any { return v.ItemAt(r) }
		default:
			fail(fmt.Errorf("unexpected QFrame column %q", name))
		}
	}
	return table{f.Len(), names, func(r, c int) any { return read[c](r) }}
}

func checksum(t table) string {
	h := sha256.New()
	for r := 0; r < t.rows; r++ {
		for c := range t.columns {
			if c > 0 {
				h.Write([]byte{'\t'})
			}
			writeValue(h, t.at(r, c))
		}
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
func writeValue(h hash.Hash, value any) {
	switch v := value.(type) {
	case string:
		h.Write([]byte(v))
	case float64:
		h.Write([]byte(strconv.FormatFloat(v, 'f', 0, 64)))
	case bool:
		h.Write([]byte(strconv.FormatBool(v)))
	default:
		fail(fmt.Errorf("unexpected value %T", value))
	}
}

// expected streams an independent formula-based oracle, preserving every row's
// order. Flag changes between repeated amount cycles to detect unstable ties.
func expected(n int, op string) (int, []string, string) {
	names := inputColumns
	count := 0
	h := sha256.New()
	row := func(i int) { fmt.Fprintf(h, "g%02d\t%d\t%t\n", i%20, i%1000, (i/1000)%2 == 0); count++ }
	switch op {
	case "select":
		names = []string{"amount"}
		for i := 0; i < n; i++ {
			fmt.Fprintf(h, "%d\n", i%1000)
			count++
		}
	case "filter":
		for i := 0; i < n; i++ {
			if i%1000 >= 500 {
				row(i)
			}
		}
	case "stable_sort":
		for amount := 999; amount >= 0; amount-- {
			for i := amount; i < n; i += 1000 {
				row(i)
			}
		}
	case "group_sum", "pipeline":
		names = []string{"group", "total"}
		sums := make([]float64, 20)
		for i := 0; i < n; i++ {
			if op == "group_sum" || i%1000 >= 500 {
				sums[i%20] += float64(i % 1000)
			}
		}
		order := make([]int, 20)
		for i := range order {
			order[i] = i
		}
		if op == "pipeline" {
			sort.SliceStable(order, func(i, j int) bool { return sums[order[i]] > sums[order[j]] })
		}
		for _, i := range order {
			fmt.Fprintf(h, "g%02d\t%.0f\n", i, sums[i])
			count++
		}
	}
	return count, names, hex.EncodeToString(h.Sum(nil))
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
