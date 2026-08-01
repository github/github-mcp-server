package thrift

import (
	"io"
)

// Features is a bitset describing the thrift encoding features supported by
// protocol implementations.
type Features uint

const (
	// DeltaEncoding is advertised by protocols that allow encoders to apply
	// delta encoding on struct fields.
	UseDeltaEncoding Features = 1 << iota

	// CoalesceBoolFields is advertised by protocols that allow encoders to
	// coalesce boolean values into field types.
	CoalesceBoolFields
)

// The Protocol interface abstracts the creation of low-level thrift readers and
// writers implementing the various protocols that the encoding supports.
//
// Protocol instances must be safe to use concurrently from multiple gourintes.
// However, the readers and writer that they instantiates are intended to be
// used by a single goroutine.
type Protocol interface {
	NewReader(r io.Reader) Reader
	NewWriter(w io.Writer) Writer
	Features() Features
}

// Reader represents a low-level reader of values encoded according to one of
// the thrift protocols.
type Reader interface {
	Protocol() Protocol
	Reader() io.Reader
	ReadBool() (bool, error)
	ReadInt8() (int8, error)
	ReadInt16() (int16, error)
	ReadInt32() (int32, error)
	ReadInt64() (int64, error)
	ReadFloat64() (float64, error)
	ReadBytes() ([]byte, error)
	ReadString() (string, error)
	ReadLength() (int, error)
	ReadMessage() (Message, error)
	ReadField() (Field, error)
	ReadList() (List, error)
	ReadSet() (Set, error)
	ReadMap() (Map, error)
}

// Writer represents a low-level writer of values encoded according to one of
// the thrift protocols.
type Writer interface {
	Protocol() Protocol
	Writer() io.Writer
	WriteBool(bool) error
	WriteInt8(int8) error
	WriteInt16(int16) error
	WriteInt32(int32) error
	WriteInt64(int64) error
	WriteFloat64(float64) error
	WriteBytes([]byte) error
	WriteString(string) error
	WriteLength(int) error
	WriteMessage(Message) error
	WriteField(Field) error
	WriteList(List) error
	WriteSet(Set) error
	WriteMap(Map) error
}
