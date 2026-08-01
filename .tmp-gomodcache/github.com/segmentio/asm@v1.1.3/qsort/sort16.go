package qsort

// A type alias is used here so that we can pass values to an avo
// generated assembly routine. At present, we can't figure out how
// to use the uint128 type in function signatures there. A type
// alias allows us to use struct { a, b uint64 } in the signature,
// and implicit conversions only work if you're using a type alias.
type uint128 = struct {
	hi uint64
	lo uint64
}

type smallsort128 func(data []uint128, base int, swap func(int, int))
type partition128 func(data []uint128, base int, swap func(int, int)) int

func quicksort128(data []uint128, base, cutoff int, smallsort smallsort128, partition partition128, swap func(int, int)) {
	for len(data) > 1 {
		if len(data) <= cutoff/16 {
			smallsort(data, base, swap)
			return
		}
		medianOfThree128(data, base, swap)
		p := partition(data, base, swap)
		if p < len(data)-p { // recurse on the smaller side
			quicksort128(data[:p], base, cutoff, smallsort, partition, swap)
			data = data[p+1:]
			base = base + p + 1
		} else {
			quicksort128(data[p+1:], base+p+1, cutoff, smallsort, partition, swap)
			data = data[:p]
		}
	}
}

func insertionsort128(data []uint128, base int, swap func(int, int)) {
	for i := 1; i < len(data); i++ {
		item := data[i]
		for j := i; j > 0 && less128(item, data[j-1]); j-- {
			data[j], data[j-1] = data[j-1], data[j]
			callswap(base, swap, j, j-1)
		}
	}
}

func medianOfThree128(data []uint128, base int, swap func(int, int)) {
	end := len(data) - 1
	mid := len(data) / 2
	if less128(data[0], data[mid]) {
		data[mid], data[0] = data[0], data[mid]
		callswap(base, swap, mid, 0)
	}
	if less128(data[end], data[0]) {
		data[0], data[end] = data[end], data[0]
		callswap(base, swap, 0, end)
		if less128(data[0], data[mid]) {
			data[mid], data[0] = data[0], data[mid]
			callswap(base, swap, mid, 0)
		}
	}
}

func hoarePartition128(data []uint128, base int, swap func(int, int)) int {
	i, j := 1, len(data)-1
	if len(data) > 0 {
		pivot := data[0]
		for j < len(data) {
			for i < len(data) && less128(data[i], pivot) {
				i++
			}
			for j > 0 && less128(pivot, data[j]) {
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

func hybridPartition128(data, scratch []uint128) int {
	pivot, lo, hi, limit := 0, 1, len(data)-1, len(scratch)

	p := distributeForward128(data, scratch, limit, lo, hi)
	if hi-p <= limit {
		scratch = scratch[limit-hi+p:]
	} else {
		lo = p + limit
		for {
			hi = distributeBackward128(data, data[lo+1-limit:], limit, lo, hi) - limit
			if hi < lo {
				p = hi
				break
			}
			lo = distributeForward128(data, data[hi+1:], limit, lo, hi) + limit
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

func less128(a, b uint128) bool {
	return a.hi < b.hi || (a.hi == b.hi && a.lo <= b.lo)
}
