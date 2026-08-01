package main

import (
	"bytes"
	"testing"

	"github.com/segmentio/encoding/json"
)

func TestIssue136(t *testing.T) {
	input := json.RawMessage(` null`)

	got, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	want := bytes.TrimSpace(input)

	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal(%q) = %q, want %q", input, got, want)
	}
}
