package xlsx

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	sf "github.com/Alifarid0011/sheetframe"
	"github.com/xuri/excelize/v2"
)

func TestRoundTrip(t *testing.T) {
	tm := time.Date(2026, 10, 28, 12, 30, 0, 0, time.UTC)
	f, err := sf.New([]string{"name", "amount", "active", "date"}, [][]any{{"=literal", 12.5, true, tm}, {"001", nil, false, nil}})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, f, "Sales"); err != nil {
		t.Fatal(err)
	}
	got, err := Read(bytes.NewReader(buf.Bytes()), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Records(), f.Records()) {
		t.Fatalf("got %v want %v", got.Records(), f.Records())
	}
	if !reflect.DeepEqual(got.Schema(), f.Schema()) {
		t.Fatal(got.Schema())
	}
}

func workbookBytes(t *testing.T, f *excelize.File) []byte {
	t.Helper()
	defer f.Close()
	b, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestReadOptionsAndDateSystem(t *testing.T) {
	f := excelize.NewFile()
	use1904 := true
	if err := f.SetWorkbookProps(&excelize.WorkbookPropsOptions{Date1904: &use1904}); err != nil {
		t.Fatal(err)
	}
	f.SetCellValue("Sheet1", "A1", "title")
	f.SetCellValue("Sheet1", "A2", "when")
	f.SetCellValue("Sheet1", "A3", 1)
	style, err := f.NewStyle(&excelize.Style{NumFmt: 14})
	if err != nil {
		t.Fatal(err)
	}
	f.SetCellStyle("Sheet1", "A3", "A3", style)
	data := workbookBytes(t, f)
	got, err := Read(bytes.NewReader(data), Options{Sheet: "Sheet1", HeaderRow: 2})
	if err != nil {
		t.Fatal(err)
	}
	v, err := got.At(0, "when")
	if err != nil {
		t.Fatal(err)
	}
	if !v.(time.Time).Equal(time.Date(1904, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatal(v)
	}
	if _, err := Read(bytes.NewReader(data), Options{Sheet: "missing"}); err == nil {
		t.Fatal("missing sheet accepted")
	}
	if _, err := Read(bytes.NewReader(data), Options{HeaderRow: -1}); err == nil {
		t.Fatal("invalid header row accepted")
	}
}

func TestReadRejectsMalformedData(t *testing.T) {
	if _, err := Read(strings.NewReader("not a workbook"), Options{}); err == nil {
		t.Fatal("invalid workbook accepted")
	}
	for _, tt := range []struct {
		name   string
		header []any
		row    []any
	}{
		{"duplicate", []any{"x", "x"}, []any{1, 2}},
		{"extra data", []any{"x"}, []any{1, 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := excelize.NewFile()
			f.SetSheetRow("Sheet1", "A1", &tt.header)
			f.SetSheetRow("Sheet1", "A2", &tt.row)
			if _, err := Read(bytes.NewReader(workbookBytes(t, f)), Options{}); err == nil {
				t.Fatal("malformed sheet accepted")
			}
		})
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteErrors(t *testing.T) {
	f, _ := sf.New([]string{"x"}, [][]any{{1}})
	sentinel := errors.New("disk full")
	if err := Write(failingWriter{sentinel}, f, "Data"); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if err := Write(&bytes.Buffer{}, f, "bad/name"); err == nil {
		t.Fatal("invalid sheet name accepted")
	}
}
