package pool_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sourcegraph/conc/pool"

	"github.com/stretchr/testify/require"
)

func ExampleResultPool() {
	p := pool.NewWithResults[int]()
	for i := 0; i < 10; i++ {
		i := i
		p.Go(func() int {
			return i * 2
		})
	}
	res := p.Wait()
	fmt.Println(res)

	// Output:
	// [0 2 4 6 8 10 12 14 16 18]
}

func TestResultGroup(t *testing.T) {
	t.Parallel()

	t.Run("panics on configuration after init", func(t *testing.T) {
		t.Run("before wait", func(t *testing.T) {
			t.Parallel()
			g := pool.NewWithResults[int]()
			g.Go(func() int { return 0 })
			require.Panics(t, func() { g.WithMaxGoroutines(10) })
		})

		t.Run("after wait", func(t *testing.T) {
			t.Parallel()
			g := pool.NewWithResults[int]()
			g.Go(func() int { return 0 })
			_ = g.Wait()
			require.Panics(t, func() { g.WithMaxGoroutines(10) })
		})
	})

	t.Run("basic", func(t *testing.T) {
		t.Parallel()
		g := pool.NewWithResults[int]()
		expected := []int{}
		for i := 0; i < 100; i++ {
			i := i
			expected = append(expected, i)
			g.Go(func() int {
				return i
			})
		}
		res := g.Wait()
		require.Equal(t, expected, res)
	})

	t.Run("deterministic order", func(t *testing.T) {
		t.Parallel()
		p := pool.NewWithResults[int]()
		results := make([]int, 100)
		for i := 0; i < 100; i++ {
			results[i] = i
		}
		for _, result := range results {
			result := result
			p.Go(func() int {
				// Add a random sleep to make it exceedingly unlikely that the
				// results are returned in the order they are submitted.
				time.Sleep(time.Duration(rand.Int()%100) * time.Millisecond)
				return result
			})
		}
		got := p.Wait()
		require.Equal(t, results, got)
	})

	t.Run("limit", func(t *testing.T) {
		t.Parallel()
		for _, maxGoroutines := range []int{1, 10, 100} {
			t.Run(strconv.Itoa(maxGoroutines), func(t *testing.T) {
				g := pool.NewWithResults[int]().WithMaxGoroutines(maxGoroutines)

				var currentConcurrent atomic.Int64
				var errCount atomic.Int64
				taskCount := maxGoroutines * 10
				expected := make([]int, taskCount)
				for i := 0; i < taskCount; i++ {
					i := i
					expected[i] = i
					g.Go(func() int {
						cur := currentConcurrent.Add(1)
						if cur > int64(maxGoroutines) {
							errCount.Add(1)
						}
						time.Sleep(time.Millisecond)
						currentConcurrent.Add(-1)
						return i
					})
				}
				res := g.Wait()
				require.Equal(t, expected, res)
				require.Equal(t, int64(0), errCount.Load())
				require.Equal(t, int64(0), currentConcurrent.Load())
			})
		}
	})

	t.Run("reuse", func(t *testing.T) {
		// Test for https://github.com/sourcegraph/conc/issues/128
		p := pool.NewWithResults[int]()

		p.Go(func() int { return 1 })
		results1 := p.Wait()
		require.Equal(t, []int{1}, results1)

		p.Go(func() int { return 2 })
		results2 := p.Wait()
		require.Equal(t, []int{2}, results2)
	})
}
