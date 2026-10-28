package playground

import (
	"encoding/json"
	"strings"
	"testing"
)

func request(t *testing.T, e *Engine, s string) Response {
	t.Helper()
	var r Response
	if err := json.Unmarshal([]byte(e.RunJSON(s)), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPipelineAndExports(t *testing.T) {
	e := &Engine{}
	r := request(t, e, `{"action":"load_csv","csv":"team,amount\nA,10\nB,30\nA,20\n"}`)
	if r.Error != "" || r.TotalRows != 3 {
		t.Fatalf("%+v", r)
	}
	r = request(t, e, `{"action":"transform","filter":{"column":"amount","op":"ge","value":20},"group":{"keys":["team"],"aggregations":[{"column":"amount","op":"sum","as":"total"}]},"sort":{"column":"total","descending":true}}`)
	if r.Error != "" || r.TotalRows != 2 || r.Rows[0][0] != "B" {
		t.Fatalf("%+v", r)
	}
	r = request(t, e, `{"action":"export_xlsx"}`)
	if r.Error != "" || r.Data == "" {
		t.Fatalf("%+v", r)
	}
	xlsx := r.Data
	r = request(t, e, `{"action":"load_xlsx","data":"`+xlsx+`"}`)
	if r.Error != "" || r.TotalRows != 2 {
		t.Fatalf("%+v", r)
	}
	r = request(t, e, `{"action":"transform","filter":{"column":"missing","op":"eq","value":1}}`)
	if !strings.Contains(r.Error, "unknown column") {
		t.Fatalf("%+v", r)
	}
	r = request(t, e, `{"action":"export_csv"}`)
	if r.Error != "" || r.Data == "" {
		t.Fatalf("failed request destroyed data: %+v", r)
	}
}

func TestInvalidRequests(t *testing.T) {
	for _, s := range []string{`{`, `{"action":"bogus"}`, `{"action":"transform"}`, `{"action":"load_xlsx","data":"%%%"}`} {
		r := request(t, &Engine{}, s)
		if r.Error == "" {
			t.Fatalf("accepted %s", s)
		}
	}
}
