package thrift_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/segmentio/encoding/thrift"
)

func TestDecodeEOF(t *testing.T) {
	p := thrift.CompactProtocol{}
	d := thrift.NewDecoder(p.NewReader(bytes.NewReader(nil)))
	v := struct{ Name string }{}

	if err := d.Decode(&v); err != io.EOF {
		t.Errorf("unexpected error returned: %v", err)
	}
}
