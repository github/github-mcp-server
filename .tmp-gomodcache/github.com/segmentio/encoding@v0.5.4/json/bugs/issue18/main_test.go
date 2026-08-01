package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/segmentio/encoding/json"
)

func TestIssue18(t *testing.T) {
	b := []byte(`{
	"userId": "blah",
	}`)

	d := json.NewDecoder(bytes.NewReader(b))

	var a struct {
		UserId string `json:"userId"`
	}
	fmt.Println(d.Decode(&a))
	fmt.Println(a)
}
