package qsort

type smallsort64 func(data []uint64, base int, swap func(int, int))
type partition64 func(data []uint64, base int, swap func(int, int)) int

func quicksort64(data []uint64, base, cutoff int, smallsort smallsort64, partition partition64, swap func(int, int)) {
	for len(data) > 1 {
		if len(data) <= cutoff/8 {
			smallsort(data, base, swap)
			return
		}
		medianOfThree64(data, base, swap)
		p := partition(data, base, swap)
		if p < len(data)-p { // recurse on the smaller side
			quicksort64(data[:p], base, cutoff, smallsort, partition, swap)
			data = data[p+1:]
			base = base + p + 1
		} else {
			quicksort64(data[p+1:], base+p+1, cutoff, smallsort, partition, swap)
			data = data[:p]
		}
	}
}

func bubblesort64NoSwap1(data []uint64, base int, swap func(int, int)) {
	for i := len(data); i > 1; i-- {
		max := data[0]

		for j := 1; j < i; j++ {
			y := data[j]
			x := uint64(0)

			if max <= y {
				x = max
			} else {
				x = y
			}

			if max <= y {
				max = y
			}

			data[j-1] = x
		}

		data[i-1] = max
	}
}

func bubblesort64NoSwap2(data []uint64, base int, swap func(int, int)) {
	for i := len(data); i > 1; i -= 2 {
		x := data[0]
		y := data[1]

		if y < x {
			x, y = y, x
		}

		for j := 2; j < i; j++ {
			z := data[j]
			w := uint64(0)
			v := uint64(0)

			if y <= z {
				w = y
			} else {
				w = z
			}

			if y <= z {
				y = z
			}

			if x <= z {
				v = x
			} else {
				v = z
			}

			if x <= z {
				x = w
			}

			data[j-2] = v
		}

		data[i-2] = x
		data[i-1] = y
	}
}

func insertionsort64(data []uint64, base int, swap func(int, int)) {
	for i := 1; i < len(data); i++ {
		item := data[i]
		for j := i; j > 0 && item < data[j-1]; j-- {
			data[j], data[j-1] = data[j-1], data[j]
			callswap(base, swap, j, j-1)
		}
	}
}

func medianOfThree64(data []uint64, base int, swap func(int, int)) {
	end := len(data) - 1
	mid := len(data) / 2
	if data[0] < data[mid] {
		data[mid], data[0] = data[0], data[mid]
		callswap(base, swap, mid, 0)
	}
	if data[end] < data[0] {
		data[0], data[end] = data[end], data[0]
		callswap(base, swap, 0, end)
		if data[0] < data[mid] {
			data[mid], data[0] = data[0], data[mid]
			callswap(base, swap, mid, 0)
		}
	}
}

func hoarePartition64(data []uint64, base int, swap func(int, int)) int {
	i, j := 1, len(data)-1
	if len(data) > 0 {
		pivot := data[0]
		for j < len(data) {
			for i < len(data) && data[i] < pivot {
				i++
			}
			for j > 0 && pivot < data[j] {
				j--
			}
			if i >= j {
				break
			}
			data[i], data[j] = data[j], data[i]
			callswap(base, swap, i, j)
			i++
			j--
		}
		data[0], data[j] = data[j], data[0]
		callswap(base, swap, 0, j)
	}
	return j
}

func hybridPartition64(data, scratch []uint64) int {
	pivot, lo, hi, limit := 0, 1, len(data)-1, len(scratch)

	p := distributeForward64(data, scratch, limit, lo, hi)
	if hi-p <= limit {
		scratch = scratch[limit-hi+p:]
	} else {
		lo = p + limit
		for {
			hi = distributeBackward64(data, data[lo+1-limit:], limit, lo, hi) - limit
			if hi < lo {
				p = hi
				break
			}
			lo = distributeForward64(data, data[hi+1:], limit, lo, hi) + limit
			if hi < lo {
				p = lo - limit
				break
			}
		}
	}

	copy(data[p+1:], scratch[:])
	data[pivot], data[p] = data[p], data[pivot]
	return p
}
