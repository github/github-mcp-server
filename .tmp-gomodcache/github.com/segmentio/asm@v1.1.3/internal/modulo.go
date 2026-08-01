package internal

func MultipleOf(size, n int) bool {
	return (isPowTwo(size) && modPowTwo(n, size) == 0) || n%size == 0
}

func PairMultipleOf(size, n, m int) bool {
	return (isPowTwo(size) && modPowTwo(n, size) == 0 && modPowTwo(m, size) == 0) ||
		(n%size == 0 && m%size == 0)
}

func isPowTwo(n int) bool {
	return modPowTwo(n, n) == 0
}

func modPowTwo(n, m int) int {
	return n & (m - 1)
}
