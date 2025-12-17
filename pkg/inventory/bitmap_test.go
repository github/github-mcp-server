package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolBitmap_SetAndIsSet(t *testing.T) {
	t.Parallel()

	var bm ToolBitmap

	// Initially empty
	assert.False(t, bm.IsSet(0))
	assert.False(t, bm.IsSet(63))
	assert.False(t, bm.IsSet(64))
	assert.False(t, bm.IsSet(127))

	// Set some bits
	bm = bm.SetBit(0)
	bm = bm.SetBit(63)
	bm = bm.SetBit(64)
	bm = bm.SetBit(127)
	bm = bm.SetBit(200)

	assert.True(t, bm.IsSet(0))
	assert.True(t, bm.IsSet(63))
	assert.True(t, bm.IsSet(64))
	assert.True(t, bm.IsSet(127))
	assert.True(t, bm.IsSet(200))

	// Unset bits should still be false
	assert.False(t, bm.IsSet(1))
	assert.False(t, bm.IsSet(62))
	assert.False(t, bm.IsSet(128))
}

func TestToolBitmap_ClearBit(t *testing.T) {
	t.Parallel()

	bm := ToolBitmap{}.SetBit(5).SetBit(10).SetBit(100)
	assert.True(t, bm.IsSet(5))
	assert.True(t, bm.IsSet(10))
	assert.True(t, bm.IsSet(100))

	bm = bm.ClearBit(10)
	assert.True(t, bm.IsSet(5))
	assert.False(t, bm.IsSet(10))
	assert.True(t, bm.IsSet(100))
}

func TestToolBitmap_Or(t *testing.T) {
	t.Parallel()

	a := ToolBitmap{}.SetBit(1).SetBit(3).SetBit(65)
	b := ToolBitmap{}.SetBit(2).SetBit(3).SetBit(130)

	result := a.Or(b)

	assert.True(t, result.IsSet(1))
	assert.True(t, result.IsSet(2))
	assert.True(t, result.IsSet(3))
	assert.True(t, result.IsSet(65))
	assert.True(t, result.IsSet(130))
	assert.False(t, result.IsSet(0))
	assert.False(t, result.IsSet(4))
}

func TestToolBitmap_And(t *testing.T) {
	t.Parallel()

	a := ToolBitmap{}.SetBit(1).SetBit(3).SetBit(5).SetBit(65)
	b := ToolBitmap{}.SetBit(3).SetBit(5).SetBit(7).SetBit(65)

	result := a.And(b)

	assert.False(t, result.IsSet(1)) // only in a
	assert.True(t, result.IsSet(3))  // in both
	assert.True(t, result.IsSet(5))  // in both
	assert.False(t, result.IsSet(7)) // only in b
	assert.True(t, result.IsSet(65)) // in both
}

func TestToolBitmap_AndNot(t *testing.T) {
	t.Parallel()

	a := ToolBitmap{}.SetBit(1).SetBit(3).SetBit(5).SetBit(65)
	b := ToolBitmap{}.SetBit(3).SetBit(7).SetBit(65)

	result := a.AndNot(b)

	assert.True(t, result.IsSet(1))   // in a, not in b
	assert.False(t, result.IsSet(3))  // in both, removed
	assert.True(t, result.IsSet(5))   // in a, not in b
	assert.False(t, result.IsSet(7))  // not in a
	assert.False(t, result.IsSet(65)) // in both, removed
}

func TestToolBitmap_PopCount(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, ToolBitmap{}.PopCount())
	assert.Equal(t, 1, ToolBitmap{}.SetBit(0).PopCount())
	assert.Equal(t, 2, ToolBitmap{}.SetBit(0).SetBit(100).PopCount())
	assert.Equal(t, 4, ToolBitmap{}.SetBit(0).SetBit(63).SetBit(64).SetBit(255).PopCount())
}

func TestToolBitmap_IsEmpty(t *testing.T) {
	t.Parallel()

	assert.True(t, ToolBitmap{}.IsEmpty())
	assert.False(t, ToolBitmap{}.SetBit(0).IsEmpty())
	assert.False(t, ToolBitmap{}.SetBit(200).IsEmpty())
}

func TestToolBitmap_Iterate(t *testing.T) {
	t.Parallel()

	bm := ToolBitmap{}.SetBit(3).SetBit(10).SetBit(64).SetBit(100).SetBit(200)

	var positions []int
	bm.Iterate(func(pos int) bool {
		positions = append(positions, pos)
		return true
	})

	assert.Equal(t, []int{3, 10, 64, 100, 200}, positions)
}

func TestToolBitmap_Iterate_EarlyStop(t *testing.T) {
	t.Parallel()

	bm := ToolBitmap{}.SetBit(1).SetBit(5).SetBit(10).SetBit(20)

	var positions []int
	bm.Iterate(func(pos int) bool {
		positions = append(positions, pos)
		return pos < 10 // stop after 10
	})

	assert.Equal(t, []int{1, 5, 10}, positions)
}

func TestToolBitmap_Positions(t *testing.T) {
	t.Parallel()

	bm := ToolBitmap{}.SetBit(5).SetBit(63).SetBit(64).SetBit(128)
	positions := bm.Positions()

	assert.Equal(t, []int{5, 63, 64, 128}, positions)
}

func TestToolBitmap_BoundaryConditions(t *testing.T) {
	t.Parallel()

	var bm ToolBitmap

	// Negative index should be no-op
	bm = bm.SetBit(-1)
	assert.False(t, bm.IsSet(-1))

	// Index >= 256 should be no-op
	bm = bm.SetBit(256)
	assert.False(t, bm.IsSet(256))

	// Maximum valid index
	bm = bm.SetBit(255)
	assert.True(t, bm.IsSet(255))
}

func BenchmarkToolBitmap_Or(b *testing.B) {
	a := ToolBitmap{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0, 0}
	c := ToolBitmap{0, 0, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.Or(c)
	}
}

func BenchmarkToolBitmap_And(b *testing.B) {
	a := ToolBitmap{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xAAAAAAAAAAAAAAAA, 0}
	c := ToolBitmap{0xAAAAAAAAAAAAAAAA, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.And(c)
	}
}

func BenchmarkToolBitmap_PopCount(b *testing.B) {
	bm := ToolBitmap{0xAAAAAAAAAAAAAAAA, 0x5555555555555555, 0xFFFFFFFF00000000, 0x00000000FFFFFFFF}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bm.PopCount()
	}
}

func BenchmarkToolBitmap_Iterate130Bits(b *testing.B) {
	// Simulate ~130 tools
	var bm ToolBitmap
	for i := 0; i < 130; i++ {
		bm = bm.SetBit(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		bm.Iterate(func(_ int) bool {
			count++
			return true
		})
	}
}
