// Package xlsx reads and writes Excel value tables for sheetframe. It uses
// Excelize for workbook compatibility; the DataFrame core has no dependencies.
// Reading uses cached formula values, not a formula engine. Writing creates a
// new workbook and does not preserve source styles, formulas, or macros.
package xlsx

import (
	"fmt"
	"io"
	"strconv"
	"time"

	sf "github.com/Alifarid0011/sheetframe"
	"github.com/xuri/excelize/v2"
)

// Options selects the worksheet and one-based header row. Zero values select
// the first worksheet and first row. Data is read into memory.
type Options struct {
	Sheet     string
	HeaderRow int
}

// Read reads a rectangular, typed worksheet. Blank cells become null; literal
// strings (including identifiers with leading zeros) stay strings. Mixed column
// types and Excel error cells are rejected with row/cell context. Built-in date
// formats 14–22 and 45–47 and ISO date cells are recognized; custom date formats
// are retained as numeric serials. Excel timestamps are interpreted in UTC.
func Read(r io.Reader, options Options) (frame sf.Frame, err error) {
	if options.HeaderRow < 0 {
		return frame, fmt.Errorf("header row must be positive")
	}
	if options.HeaderRow == 0 {
		options.HeaderRow = 1
	}
	f, err := excelize.OpenReader(r)
	if err != nil {
		return frame, fmt.Errorf("open XLSX: %w", err)
	}
	defer func() {
		if e := f.Close(); err == nil && e != nil {
			err = e
		}
	}()
	sheet := options.Sheet
	if sheet == "" {
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return frame, fmt.Errorf("workbook has no sheets")
		}
		sheet = sheets[0]
	}
	iterator, err := f.Rows(sheet)
	if err != nil {
		return frame, fmt.Errorf("sheet %q: %w", sheet, err)
	}
	defer iterator.Close()
	props, err := f.GetWorkbookProps()
	if err != nil {
		return frame, err
	}
	date1904 := props.Date1904 != nil && *props.Date1904
	styles := map[int]bool{}
	var headers []string
	rows := [][]any{}
	rowNumber := 0
	for iterator.Next() {
		rowNumber++
		if rowNumber < options.HeaderRow {
			continue
		}
		cells, e := iterator.Columns(excelize.Options{RawCellValue: true})
		if e != nil {
			return frame, fmt.Errorf("row %d: %w", rowNumber, e)
		}
		if rowNumber == options.HeaderRow {
			headers = append([]string(nil), cells...)
			if len(headers) == 0 {
				return frame, fmt.Errorf("header row is empty")
			}
			continue
		}
		if len(cells) > len(headers) {
			for _, v := range cells[len(headers):] {
				if v != "" {
					return frame, fmt.Errorf("row %d has data beyond the %d headers", rowNumber, len(headers))
				}
			}
		}
		row := make([]any, len(headers))
		for c := range headers {
			cell, e := excelize.CoordinatesToCellName(c+1, rowNumber)
			if e != nil {
				return frame, e
			}
			raw := ""
			if c < len(cells) {
				raw = cells[c]
			}
			value, e := readCell(f, sheet, cell, raw, date1904, styles)
			if e != nil {
				return frame, fmt.Errorf("sheet %q cell %s: %w", sheet, cell, e)
			}
			row[c] = value
		}
		rows = append(rows, row)
	}
	if e := iterator.Error(); e != nil {
		return frame, e
	}
	if headers == nil {
		return frame, fmt.Errorf("header row %d not found", options.HeaderRow)
	}
	frame, err = sf.New(headers, rows)
	if err != nil {
		return frame, fmt.Errorf("sheet %q (data row indices are zero-based): %w", sheet, err)
	}
	return frame, nil
}

func readCell(f *excelize.File, sheet, cell, raw string, date1904 bool, styles map[int]bool) (any, error) {
	typ, err := f.GetCellType(sheet, cell)
	if err != nil {
		return nil, err
	}
	switch typ {
	case excelize.CellTypeInlineString, excelize.CellTypeSharedString:
		return raw, nil
	case excelize.CellTypeError:
		return nil, fmt.Errorf("Excel error %s", raw)
	case excelize.CellTypeBool:
		if raw == "1" || raw == "true" {
			return true, nil
		}
		if raw == "0" || raw == "false" {
			return false, nil
		}
		return nil, fmt.Errorf("invalid boolean %q", raw)
	case excelize.CellTypeDate:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02"} {
			if tm, e := time.Parse(layout, raw); e == nil {
				return tm.UTC(), nil
			}
		}
		return nil, fmt.Errorf("invalid ISO date %q", raw)
	}
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid numeric cell %q: %w", raw, err)
	}
	styleID, err := f.GetCellStyle(sheet, cell)
	if err != nil {
		return nil, err
	}
	isDate, ok := styles[styleID]
	if !ok {
		style, e := f.GetStyle(styleID)
		if e != nil {
			return nil, e
		}
		num := style.NumFmt
		isDate = (num >= 14 && num <= 22) || (num >= 45 && num <= 47)
		styles[styleID] = isDate
	}
	if isDate {
		return excelize.ExcelDateToTime(n, date1904)
	}
	return n, nil
}

// Write creates an XLSX workbook with literal text, numeric, boolean, date, and
// null cells. Dates are stored as Excel UTC serials with a date/time style and
// are subject to Excel's supported date range and precision. The caller's
// writer is not closed. A blank sheet name selects "Sheet1".
func Write(w io.Writer, frame sf.Frame, sheet string) (err error) {
	headers := frame.Columns()
	if len(headers) == 0 {
		return fmt.Errorf("XLSX export requires at least one column")
	}
	if sheet == "" {
		sheet = "Sheet1"
	}
	f := excelize.NewFile()
	defer func() {
		if e := f.Close(); err == nil && e != nil {
			err = e
		}
	}()
	if sheet != "Sheet1" {
		if err := f.SetSheetName("Sheet1", sheet); err != nil {
			return err
		}
	}
	dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 22})
	if err != nil {
		return err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	stream, err := f.NewStreamWriter(sheet)
	if err != nil {
		return err
	}
	header := make([]any, len(headers))
	for i, n := range headers {
		header[i] = excelize.Cell{StyleID: headerStyle, Value: n}
	}
	if err := stream.SetRow("A1", header); err != nil {
		return err
	}
	n, _ := frame.Shape()
	for r := 0; r < n; r++ {
		row := make([]any, len(headers))
		for c, name := range headers {
			v, e := frame.At(r, name)
			if e != nil {
				return e
			}
			if tm, ok := v.(time.Time); ok {
				v = excelize.Cell{StyleID: dateStyle, Value: tm.UTC()}
			}
			row[c] = v
		}
		cell, e := excelize.CoordinatesToCellName(1, r+2)
		if e != nil {
			return e
		}
		if e := stream.SetRow(cell, row); e != nil {
			return e
		}
	}
	if err := stream.Flush(); err != nil {
		return err
	}
	return f.Write(w)
}
