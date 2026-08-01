package mem

import (
	"fmt"
	"testing"
)

var (
	countPairSizes = [...]int{
		1, 2, 4, 8, 10, 16, 32,
	}
)

func TestCountPair(t *testing.T) {
	for _, size := range countPairSizes {
		makeInput := func(values ...byte) []byte {
			input := make([]byte, size*len(values))
			for i := range values {
				input[i*size] = values[i]
			}
			return input
		}

		t.Run(fmt.Sprintf("N=%d", size), func(t *testing.T) {
			tests := []struct {
				scenario string
				input    []byte
				count    int
			}{
				{
					scenario: "empty input",
					input:    nil,
					count:    0,
				},

				{
					scenario: "input with only one item",
					input:    makeInput(1),
					count:    0,
				},

				{
					scenario: "input with two non-equal items",
					input:    makeInput(1, 2),
					count:    0,
				},

				{
					scenario: "input with two equal items",
					input:    makeInput(1, 1),
					count:    1,
				},

				{
					scenario: "input with two equal items in the middle",
					input:    makeInput(0, 1, 2, 3, 4, 5, 5, 6, 7, 8, 9),
					count:    1,
				},

				{
					scenario: "input with two equal items at the end",
					input:    makeInput(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 9),
					count:    1,
				},

				{
					scenario: "input with many equal items at the beginning of a long sequence",
					input: makeInput(
						0, 0, 0, 0, 0, 0, 0, 0, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
					),
					count: 9,
				},

				{
					scenario: "input with many equal items in the middle of a long sequence",
					input: makeInput(
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 0, 0, 0, 0, 0, 0, 0, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
						0, 1, 2, 3, 4, 5, 6, 7, 8,
					),
					count: 9,
				},

				{
					scenario: "input with many equal items in a long sequence",
					input: makeInput(
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
						0, 1, 2, 3, 4, 5, 6, 7, 0,
					),
					count: 15,
				},
			}

			for _, test := range tests {
				t.Run(test.scenario, func(t *testing.T) {
					n := CountPair(test.input, size)

					if n != test.count {
						t.Errorf("expected=%d found=%d", test.count, n)
					}
				})
			}
		})
	}
}

func BenchmarkCountPair(b *testing.B) {
	for _, size := range countPairSizes {
		input := make([]byte, 16*1024)
		for i := range input {
			input[i] = byte(i)
		}

		if size%len(input) != 0 {
			input = input[:(len(input)/size)*size]
		}

		b.Run(fmt.Sprintf("N=%d", size), func(b *testing.B) {
			b.SetBytes(int64(len(input)))

			for i := 0; i < b.N; i++ {
				n := CountPair(input, size)
				if n != 0 {
					b.Fatal("unexpected result:", n)
				}
			}
		})
	}
}
