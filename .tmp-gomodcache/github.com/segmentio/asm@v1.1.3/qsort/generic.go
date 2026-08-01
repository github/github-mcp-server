package qsort

import "bytes"

type generic struct {
	data []byte
	size int
	temp []byte
	swap func(int, int)
}

func newGeneric(data []byte, size int, swap func(int, int)) *generic {
	return &generic{
		data: data,
		size: size,
		temp: make([]byte, size),
		swap: swap,
	}
}

func (g *generic) Len() int {
	return len(g.data) / g.size
}

func (g *generic) Less(i, j int) bool {
	return bytes.Compare(g.slice(i), g.slice(j)) < 0
}

func (g *generic) Swap(i, j int) {
	copy(g.temp, g.slice(j))
	copy(g.slice(j), g.slice(i))
	copy(g.slice(i), g.temp)
	if g.swap != nil {
		g.swap(i, j)
	}
}

func (g *generic) slice(i int) []byte {
	return g.data[i*g.size : (i+1)*g.size]
}
