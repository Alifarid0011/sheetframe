package sheetframe

import "math"

// sortNumbers stably orders finite float64 values by their IEEE bit patterns.
// Sign adjustment makes unsigned key order match numeric order. The largest
// key is reserved for null, in either direction; New rejects NaN and infinities.
// Constant bytes are skipped, but arbitrary finite values use at most 8 passes.
func sortNumbers(values []any, indices []int, descending bool) []int {
	keys := make([]uint64, len(values))
	var varying uint64
	ordered := true
	for i, value := range values {
		key := ^uint64(0)
		if value != nil {
			bits := math.Float64bits(value.(float64))
			if bits>>63 != 0 {
				key = ^bits
			} else {
				key = bits ^ (uint64(1) << 63)
			}
			if descending {
				key = ^key
			}
		}
		keys[i] = key
		varying |= key ^ keys[0]
		if i > 0 && key < keys[i-1] {
			ordered = false
		}
	}
	if ordered || varying == 0 {
		return indices
	}
	scratch := make([]int, len(indices))
	for shift := uint(0); shift < 64; shift += 8 {
		if byte(varying>>shift) == 0 {
			continue
		}
		var offsets [256]int
		for _, row := range indices {
			offsets[byte(keys[row]>>shift)]++
		}
		start := 0
		for bucket, count := range offsets {
			offsets[bucket], start = start, start+count
		}
		for _, row := range indices {
			bucket := byte(keys[row] >> shift)
			scratch[offsets[bucket]] = row
			offsets[bucket]++
		}
		indices, scratch = scratch, indices
	}
	return indices
}
