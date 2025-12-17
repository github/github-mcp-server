package inventory

import (
	"math/bits"
)

// ToolBitmap represents a set of tools as a bitmap for O(1) set operations.
// Uses 4 uint64s (256 bits) to support up to 256 tools.
// Current tool count is ~130, so this provides headroom for growth.
type ToolBitmap [4]uint64

// EmptyBitmap returns an empty bitmap with no bits set.
func EmptyBitmap() ToolBitmap {
	return ToolBitmap{}
}

// SetBit returns a new bitmap with the bit at position i set.
func (b ToolBitmap) SetBit(i int) ToolBitmap {
	if i < 0 || i >= 256 {
		return b
	}
	word, bit := i/64, uint(i%64) //nolint:gosec // bounds checked above
	result := b
	result[word] |= 1 << bit
	return result
}

// ClearBit returns a new bitmap with the bit at position i cleared.
func (b ToolBitmap) ClearBit(i int) ToolBitmap {
	if i < 0 || i >= 256 {
		return b
	}
	word, bit := i/64, uint(i%64) //nolint:gosec // bounds checked above
	result := b
	result[word] &^= 1 << bit
	return result
}

// IsSet returns true if the bit at position i is set.
func (b ToolBitmap) IsSet(i int) bool {
	if i < 0 || i >= 256 {
		return false
	}
	word, bit := i/64, uint(i%64) //nolint:gosec // bounds checked above
	return (b[word] & (1 << bit)) != 0
}

// Or returns the union of two bitmaps.
func (b ToolBitmap) Or(other ToolBitmap) ToolBitmap {
	return ToolBitmap{
		b[0] | other[0],
		b[1] | other[1],
		b[2] | other[2],
		b[3] | other[3],
	}
}

// And returns the intersection of two bitmaps.
func (b ToolBitmap) And(other ToolBitmap) ToolBitmap {
	return ToolBitmap{
		b[0] & other[0],
		b[1] & other[1],
		b[2] & other[2],
		b[3] & other[3],
	}
}

// AndNot returns b AND NOT other (bits in b that are not in other).
func (b ToolBitmap) AndNot(other ToolBitmap) ToolBitmap {
	return ToolBitmap{
		b[0] &^ other[0],
		b[1] &^ other[1],
		b[2] &^ other[2],
		b[3] &^ other[3],
	}
}

// PopCount returns the number of set bits (population count).
func (b ToolBitmap) PopCount() int {
	return bits.OnesCount64(b[0]) +
		bits.OnesCount64(b[1]) +
		bits.OnesCount64(b[2]) +
		bits.OnesCount64(b[3])
}

// IsEmpty returns true if no bits are set.
func (b ToolBitmap) IsEmpty() bool {
	return b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0
}

// Iterate calls fn for each set bit position. Stops if fn returns false.
func (b ToolBitmap) Iterate(fn func(position int) bool) {
	for word := 0; word < 4; word++ {
		v := b[word]
		base := word * 64
		for v != 0 {
			// Find position of lowest set bit
			tz := bits.TrailingZeros64(v)
			if !fn(base + tz) {
				return
			}
			// Clear the lowest set bit
			v &= v - 1
		}
	}
}

// Positions returns a slice of all set bit positions.
func (b ToolBitmap) Positions() []int {
	result := make([]int, 0, b.PopCount())
	b.Iterate(func(pos int) bool {
		result = append(result, pos)
		return true
	})
	return result
}
