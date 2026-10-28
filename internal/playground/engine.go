// Package playground contains the stateful, browser-demo request handler.
package playground

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sf "github.com/Alifarid0011/sheetframe"
	"github.com/Alifarid0011/sheetframe/xlsx"
)

type filter struct {
	Column string `json:"column"`
	Op     sf.Op  `json:"op"`
	Value  any    `json:"value"`
}
type sorting struct {
	Column     string `json:"column"`
	Descending bool   `json:"descending"`
}
type grouping struct {
	Keys         []string         `json:"keys"`
	Aggregations []sf.Aggregation `json:"aggregations"`
}
type requestData struct {
	Action string    `json:"action"`
	CSV    string    `json:"csv"`
	Data   string    `json:"data"`
	Sheet  string    `json:"sheet"`
	Filter *filter   `json:"filter"`
	Sort   *sorting  `json:"sort"`
	Group  *grouping `json:"group"`
}

// Response contains a bounded preview; exports use every result row.
type Response struct {
	Columns   []string   `json:"columns"`
	Schema    []sf.Field `json:"schema"`
	Rows      [][]any    `json:"rows"`
	TotalRows int        `json:"totalRows"`
	ElapsedMS float64    `json:"elapsedMS"`
	Error     string     `json:"error,omitempty"`
	Data      string     `json:"data,omitempty"`
}

// Engine holds one local demo session. Call it serially, as the browser does.
type Engine struct {
	base, current sf.Frame
	loaded        bool
}

// RunJSON handles a single JSON request, always returning a JSON response.
// Failed requests leave the previous successful dataset available.
func (e *Engine) RunJSON(input string) string {
	start := time.Now()
	var req requestData
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return errorJSON(err)
	}
	response, err := e.run(req)
	if err != nil {
		return errorJSON(err)
	}
	response.ElapsedMS = float64(time.Since(start).Nanoseconds()) / 1e6
	data, err := json.Marshal(response)
	if err != nil {
		return errorJSON(err)
	}
	return string(data)
}

func errorJSON(err error) string {
	b, _ := json.Marshal(Response{Error: err.Error()})
	return string(b)
}

func (e *Engine) run(req requestData) (Response, error) {
	var next sf.Frame
	var err error
	switch req.Action {
	case "load_csv":
		next, err = sf.ReadCSV(strings.NewReader(req.CSV))
	case "load_xlsx":
		var data []byte
		data, err = base64.StdEncoding.DecodeString(req.Data)
		if err == nil {
			next, err = xlsx.Read(bytes.NewReader(data), xlsx.Options{Sheet: req.Sheet})
		}
	case "transform":
		if !e.loaded {
			return Response{}, fmt.Errorf("load a dataset first")
		}
		next = e.base
		if q := req.Filter; q != nil {
			value := q.Value
			for _, field := range next.Schema() {
				if field.Name == q.Column && field.Kind == sf.Time && q.Op != sf.IsNull && q.Op != sf.NotNull {
					if s, ok := value.(string); ok {
						value, err = time.Parse(time.RFC3339Nano, s)
					}
				}
			}
			if err == nil {
				next, err = next.Filter(q.Column, q.Op, value)
			}
		}
		if err == nil && req.Group != nil {
			var group sf.Grouped
			group, err = next.GroupBy(req.Group.Keys...)
			if err == nil {
				next, err = group.Aggregate(req.Group.Aggregations...)
			}
		}
		if err == nil && req.Sort != nil {
			next, err = next.Sort(req.Sort.Column, req.Sort.Descending)
		}
	case "export_csv", "export_xlsx":
		if !e.loaded {
			return Response{}, fmt.Errorf("load a dataset first")
		}
		var buf bytes.Buffer
		if req.Action == "export_csv" {
			err = e.current.WriteCSV(&buf)
		} else {
			err = xlsx.Write(&buf, e.current, "Results")
		}
		if err != nil {
			return Response{}, err
		}
		response := preview(e.current)
		response.Data = base64.StdEncoding.EncodeToString(buf.Bytes())
		return response, nil
	default:
		return Response{}, fmt.Errorf("unknown action %q", req.Action)
	}
	if err != nil {
		return Response{}, err
	}
	if req.Action == "load_csv" || req.Action == "load_xlsx" {
		e.base = next
		e.loaded = true
	}
	e.current = next
	return preview(next), nil
}

func preview(f sf.Frame) Response {
	n, _ := f.Shape()
	return Response{Columns: f.Columns(), Schema: f.Schema(), Rows: f.Head(200).Records(), TotalRows: n}
}
