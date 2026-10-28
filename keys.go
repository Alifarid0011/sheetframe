package sheetframe

import (
	"encoding/binary"
	"math"
	"time"
)

// rowKeys reuses encoding space for compound keys. A single column already
// contains comparable, validated values, so its common path needs no encoding.
type rowKeys struct {
	frame Frame
	cols  []int
	buf   []byte
}

func (k *rowKeys) at(row int) (any, bool) {
	if len(k.cols) == 1 {
		v := k.frame.columns[k.cols[0]].values[row]
		if tm, ok := v.(time.Time); ok {
			v = tm.UTC() // Remove location and monotonic metadata.
		}
		return v, v == nil
	}
	b := k.buf[:0]
	hasNull := false
	for _, c := range k.cols {
		switch v := k.frame.columns[c].values[row].(type) {
		case nil:
			b = append(b, 0)
			hasNull = true
		case string:
			b = append(b, 1)
			b = binary.AppendUvarint(b, uint64(len(v)))
			b = append(b, v...)
		case float64:
			b = append(b, 2)
			b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
		case bool:
			b = append(b, 3)
			if v {
				b = append(b, 1)
			} else {
				b = append(b, 0)
			}
		case time.Time:
			v = v.UTC()
			year, month, day := v.Date()
			hour, minute, second := v.Clock()
			b = append(b, 4)
			b = binary.AppendVarint(b, int64(year))
			b = append(b, byte(month), byte(day), byte(hour), byte(minute), byte(second))
			b = binary.LittleEndian.AppendUint32(b, uint32(v.Nanosecond()))
		}
	}
	k.buf = b
	return string(b), hasNull
}

func (k *rowKeys) values(row int) []any {
	values := make([]any, len(k.cols))
	for i, c := range k.cols {
		v := k.frame.columns[c].values[row]
		if tm, ok := v.(time.Time); ok {
			v = tm.UTC()
		}
		values[i] = v
	}
	return values
}
