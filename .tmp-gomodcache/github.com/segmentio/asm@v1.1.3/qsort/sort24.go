package qsort

type uint192 = struct {
	hi  uint64
	mid uint64
	lo  uint64
}

type smallsort192 func(data []uint192, base int, swap func(int, int))
type partition192 func(data []uint192, base int, swap func(int, int)) int

func quicksort192(data []uint192, base, cutoff int, smallsort smallsort192, partition partition192, swap func(int, int)) {
	for len(data) > 1 {
		if len(data) <= cutoff/24 {
			smallsort(data, base, swap)
			return
		}
		medianOfThree192(data, base, swap)
		p := partition(data, base, swap)
		if p < len(data)-p { // recurse on the smaller side
			quicksort192(data[:p], base, cutoff, smallsort, partition, swap)
			data = data[p+1:]
			base = base + p + 1
		} else {
			quicksort192(data[p+1:], base+p+1, cutoff, smallsort, partition, swap)
			data = data[:p]
		}
	}
}

func insertionsort192(data []uint192, base int, swap func(int, int)) {
	for i := 1; i < len(data); i++ {
		item := data[i]
		for j := i; j > 0 && less192(item, data[j-1]); j-- {
			data[j], data[j-1] = data[j-1], data[j]
			callswap(base, swap, j, j-1)
		}
	}
}

func medianOfThree192(data []uint192, base int, swap func(int, int)) {
	end := len(data) - 1
	mid := len(data) / 2
	if less192(data[0], data[mid]) {
		data[mid], data[0] = data[0], data[mid]
		callswap(base, swap, mid, 0)
	}
	if less192(data[end], data[0]) {
		data[0], data[end] = data[end], data[0]
		callswap(base, swap, 0, end)
		if less192(data[0], data[mid]) {
			data[mid], data[0] = data[0], data[mid]
			callswap(base, swap, mid, 0)
		}
	}
}

func hoarePartition192(data []uint192, base int, swap func(int, int)) int {
	i, j := 1, len(data)-1
	if len(data) > 0 {
		pivot := data[0]
		for j < len(data) {
			for i < len(data) && less192(data[i], pivot) {
				i++
			}
			for j > 0 && less192(pivot, data[j]) {
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

func less192(a, b uint192) bool {
	return a.hi < b.hi ||
		(a.hi == b.hi && a.mid < b.mid) ||
		(a.hi == b.hi && a.mid == b.mid && a.lo <= b.lo)
}
