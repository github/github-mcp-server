package main

import (
	"fmt"
	"log"
	"testing"

	"github.com/segmentio/encoding/json"
)

func TestIssue11(t *testing.T) {
	m := map[string]map[string]any{
		"outerkey": {
			"innerkey": "innervalue",
		},
	}

	b, err := json.Marshal(m)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))
}
