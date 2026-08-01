package thrift

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// CompactProtocol is a Protocol implementation for the compact thrift protocol.
//
// https://github.com/apache/thrift/blob/master/doc/specs/thrift-compact-protocol.md#integer-encoding
type CompactProtocol struct{}

func (p *CompactProtocol) NewReader(r io.Reader) Reader {
	return &compactReader{protocol: p, binary: binaryReader{r: r}}
}

func (p *CompactProtocol) NewWriter(w io.Writer) Writer {
	return &compactWriter{protocol: p, binary: binaryWriter{w: w}}
}

func (p *CompactProtocol) Features() Features {
	return UseDeltaEncoding | CoalesceBoolFields
}

type compactReader struct {
	protocol *CompactProtocol
	binary   binaryReader
}

func (r *compactReader) Protocol() Protocol {
	return r.protocol
}

func (r *compactReader) Reader() io.Reader {
	return r.binary.Reader()
}

func (r *compactReader) ReadBool() (bool, error) {
	return r.binary.ReadBool()
}

func (r *compactReader) ReadInt8() (int8, error) {
	return r.binary.ReadInt8()
}

func (r *compactReader) ReadInt16() (int16, error) {
	v, err := r.readVarint("int16", math.MinInt16, math.MaxInt16)
	return int16(v), err
}

func (r *compactReader) ReadInt32() (int32, error) {
	v, err := r.readVarint("int32", math.MinInt32, math.MaxInt32)
	return int32(v), err
}

func (r *compactReader) ReadInt64() (int64, error) {
	return r.readVarint("int64", math.MinInt64, math.MaxInt64)
}

func (r *compactReader) ReadFloat64() (float64, error) {
	return r.binary.ReadFloat64()
}

func (r *compactReader) ReadBytes() ([]byte, error) {
	n, err := r.ReadLength()
	if err != nil {
		return nil, err
	}
	b := make([]byte, n)
	_, err = io.ReadFull(r.Reader(), b)
	return b, err
}

func (r *compactReader) ReadString() (string, error) {
	b, err := r.ReadBytes()
	return unsafeBytesToString(b), err
}

func (r *compactReader) ReadLength() (int, error) {
	n, err := r.readUvarint("length", math.MaxInt32)
	return int(n), err
}

func (r *compactReader) ReadMessage() (Message, error) {
	m := Message{}

	b0, err := r.ReadByte()
	if err != nil {
		return m, err
	}
	if b0 != 0x82 {
		return m, fmt.Errorf("invalid protocol id found when reading thrift message: %#x", b0)
	}

	b1, err := r.ReadByte()
	if err != nil {
		return m, dontExpectEOF(err)
	}

	seqID, err := r.readUvarint("seq id", math.MaxInt32)
	if err != nil {
		return m, dontExpectEOF(err)
	}

	m.Type = MessageType(b1) & 0x7
	m.SeqID = int32(seqID)
	m.Name, err = r.ReadString()
	return m, dontExpectEOF(err)
}

func (r *compactReader) ReadField() (Field, error) {
	f := Field{}

	b, err := r.ReadByte()
	if err != nil {
		return f, err
	}

	if Type(b) == STOP {
		return f, nil
	}

	if (b >> 4) != 0 {
		f = Field{ID: int16(b >> 4), Type: Type(b & 0xF), Delta: true}
	} else {
		i, err := r.ReadInt16()
		if err != nil {
			return f, dontExpectEOF(err)
		}
		f = Field{ID: i, Type: Type(b)}
	}

	return f, nil
}

func (r *compactReader) ReadList() (List, error) {
	b, err := r.ReadByte()
	if err != nil {
		return List{}, err
	}
	if (b >> 4) != 0xF {
		return List{Size: int32(b >> 4), Type: Type(b & 0xF)}, nil
	}
	n, err := r.readUvarint("list size", math.MaxInt32)
	if err != nil {
		return List{}, dontExpectEOF(err)
	}
	return List{Size: int32(n), Type: Type(b & 0xF)}, nil
}

func (r *compactReader) ReadSet() (Set, error) {
	l, err := r.ReadList()
	return Set(l), err
}

func (r *compactReader) ReadMap() (Map, error) {
	n, err := r.readUvarint("map size", math.MaxInt32)
	if err != nil {
		return Map{}, err
	}
	if n == 0 { // empty map
		return Map{}, nil
	}
	b, err := r.ReadByte()
	if err != nil {
		return Map{}, dontExpectEOF(err)
	}
	return Map{Size: int32(n), Key: Type(b >> 4), Value: Type(b & 0xF)}, nil
}

func (r *compactReader) ReadByte() (byte, error) {
	return r.binary.ReadByte()
}

func (r *compactReader) readUvarint(typ string, max uint64) (uint64, error) {
	var br io.ByteReader

	switch x := r.Reader().(type) {
	case *bytes.Buffer:
		br = x
	case *bytes.Reader:
		br = x
	case *bufio.Reader:
		br = x
	case io.ByteReader:
		br = x
	default:
		br = &r.binary
	}

	u, err := binary.ReadUvarint(br)
	if err == nil {
		if u > max {
			err = fmt.Errorf("%s varint out of range: %d > %d", typ, u, max)
		}
	}
	return u, err
}

func (r *compactReader) readVarint(typ string, min, max int64) (int64, error) {
	var br io.ByteReader

	switch x := r.Reader().(type) {
	case *bytes.Buffer:
		br = x
	case *bytes.Reader:
		br = x
	case *bufio.Reader:
		br = x
	case io.ByteReader:
		br = x
	default:
		br = &r.binary
	}

	v, err := binary.ReadVarint(br)
	if err == nil {
		if v < min || v > max {
			err = fmt.Errorf("%s varint out of range: %d not in [%d;%d]", typ, v, min, max)
		}
	}
	return v, err
}

type compactWriter struct {
	protocol *CompactProtocol
	binary   binaryWriter
	varint   [binary.MaxVarintLen64]byte
}

func (w *compactWriter) Protocol() Protocol {
	return w.protocol
}

func (w *compactWriter) Writer() io.Writer {
	return w.binary.Writer()
}

func (w *compactWriter) WriteBool(v bool) error {
	return w.binary.WriteBool(v)
}

func (w *compactWriter) WriteInt8(v int8) error {
	return w.binary.WriteInt8(v)
}

func (w *compactWriter) WriteInt16(v int16) error {
	return w.writeVarint(int64(v))
}

func (w *compactWriter) WriteInt32(v int32) error {
	return w.writeVarint(int64(v))
}

func (w *compactWriter) WriteInt64(v int64) error {
	return w.writeVarint(v)
}

func (w *compactWriter) WriteFloat64(v float64) error {
	return w.binary.WriteFloat64(v)
}

func (w *compactWriter) WriteBytes(v []byte) error {
	if err := w.WriteLength(len(v)); err != nil {
		return err
	}
	return w.binary.write(v)
}

func (w *compactWriter) WriteString(v string) error {
	if err := w.WriteLength(len(v)); err != nil {
		return err
	}
	return w.binary.writeString(v)
}

func (w *compactWriter) WriteLength(n int) error {
	if n < 0 {
		return fmt.Errorf("negative length cannot be encoded in thrift: %d", n)
	}
	if n > math.MaxInt32 {
		return fmt.Errorf("length is too large to be encoded in thrift: %d", n)
	}
	return w.writeUvarint(uint64(n))
}

func (w *compactWriter) WriteMessage(m Message) error {
	if err := w.binary.writeByte(0x82); err != nil {
		return err
	}
	if err := w.binary.writeByte(byte(m.Type)); err != nil {
		return err
	}
	if err := w.writeUvarint(uint64(m.SeqID)); err != nil {
		return err
	}
	return w.WriteString(m.Name)
}

func (w *compactWriter) WriteField(f Field) error {
	if f.Type == STOP {
		return w.binary.writeByte(0)
	}
	if f.ID <= 15 {
		return w.binary.writeByte(byte(f.ID<<4) | byte(f.Type))
	}
	if err := w.binary.writeByte(byte(f.Type)); err != nil {
		return err
	}
	return w.WriteInt16(f.ID)
}

func (w *compactWriter) WriteList(l List) error {
	if l.Size <= 14 {
		return w.binary.writeByte(byte(l.Size<<4) | byte(l.Type))
	}
	if err := w.binary.writeByte(0xF0 | byte(l.Type)); err != nil {
		return err
	}
	return w.writeUvarint(uint64(l.Size))
}

func (w *compactWriter) WriteSet(s Set) error {
	return w.WriteList(List(s))
}

func (w *compactWriter) WriteMap(m Map) error {
	if err := w.writeUvarint(uint64(m.Size)); err != nil || m.Size == 0 {
		return err
	}
	return w.binary.writeByte((byte(m.Key) << 4) | byte(m.Value))
}

func (w *compactWriter) writeUvarint(v uint64) error {
	n := binary.PutUvarint(w.varint[:], v)
	return w.binary.write(w.varint[:n])
}

func (w *compactWriter) writeVarint(v int64) error {
	n := binary.PutVarint(w.varint[:], v)
	return w.binary.write(w.varint[:n])
}
