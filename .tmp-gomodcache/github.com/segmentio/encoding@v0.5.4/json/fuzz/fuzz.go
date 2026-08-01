//go:build ignore
// +build ignore

// Copyright 2015 go-fuzz project authors. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package fuzz

import (
	"bytes"
	encodingJSON "encoding/json"
	"fmt"
	"reflect"

	"github.com/dvyukov/go-fuzz-corpus/fuzz"
	"github.com/segmentio/encoding/json"
)

func fixS(v any) {
	if s, ok := v.(*S); ok {
		if len(s.P) == 0 {
			s.P = []byte(`""`)
		}
	}
}

func Fuzz(data []byte) int {
	score := 0
	for _, ctor := range []func() any{
		func() any { return nil },
		func() any { return new([]any) },
		func() any { m := map[string]string{}; return &m },
		func() any { m := map[string]any{}; return &m },
		func() any { return new(S) },
	} {
		// Note: we modified the test to verify that we behavior like the
		// standard encoding/json package, whether it's right or wrong.
		v1 := ctor()
		v2 := ctor()

		err1 := encodingJSON.Unmarshal(data, v1)
		err2 := json.Unmarshal(data, v2)

		if err1 != nil {
			if err2 != nil {
				// both implementations report an error
				if reflect.TypeOf(err1) != reflect.TypeOf(err2) {
					fmt.Printf("input: %s\n", string(data))
					fmt.Printf("encoding/json.Unmarshal(%T): %T: %s\n", v1, err1, err1)
					fmt.Printf("segmentio/encoding/json.Unmarshal(%T): %T: %s\n", v2, err2, err2)
					panic("error types mismatch")
				}
				continue
			} else {
				fmt.Printf("input: %s\n", string(data))
				fmt.Printf("encoding/json.Unmarshal(%T): %T: %s\n", v1, err1, err1)
				fmt.Printf("segmentio/encoding/json.Unmarshal(%T): <nil>\n")
				panic("error values mismatch")
			}
		} else {
			if err2 != nil {
				fmt.Printf("input: %s\n", string(data))
				fmt.Printf("encoding/json.Unmarshal(%T): <nil>\n")
				fmt.Printf("segmentio/encoding/json.Unmarshal(%T): %T: %s\n", v2, err2, err2)
				panic("error values mismatch")
			} else {
				// both implementations pass
			}
		}

		score = 1
		fixS(v1)
		fixS(v2)
		if !fuzz.DeepEqual(v1, v2) {
			fmt.Printf("input: %s\n", string(data))
			fmt.Printf("encoding/json:      %#v\n", v1)
			fmt.Printf("segmentio/encoding: %#v\n", v2)
			panic("not equal")
		}

		data1, err := encodingJSON.Marshal(v1)
		if err != nil {
			panic(err)
		}
		data2, err := json.Marshal(v2)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(data1, data2) {
			fmt.Printf("input: %s\n", string(data))
			fmt.Printf("encoding/json:      %s\n", string(data1))
			fmt.Printf("segmentio/encoding: %s\n", string(data2))
			panic("not equal")
		}
	}
	return score
}

type S struct {
	A int    `json:",omitempty"`
	B string `json:"B1,omitempty"`
	C float64
	D bool
	E uint8
	F []byte
	G any
	H map[string]any
	I map[string]string
	J []any
	K []string
	L S1
	M *S1
	N *int
	O **int
	P json.RawMessage
	Q Marshaller
	R int `json:"-"`
	S int `json:",string"`
}

type S1 struct {
	A int
	B string
}

type Marshaller struct {
	v string
}

func (m *Marshaller) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.v)
}

func (m *Marshaller) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &m.v)
}
