package thrift

import (
	"io"
	"log"
)

func NewDebugReader(r Reader, l *log.Logger) Reader {
	return &debugReader{
		r: r,
		l: l,
	}
}

func NewDebugWriter(w Writer, l *log.Logger) Writer {
	return &debugWriter{
		w: w,
		l: l,
	}
}

type debugReader struct {
	r Reader
	l *log.Logger
}

func (d *debugReader) log(method string, res any, err error) {
	if err != nil {
		d.l.Printf("(%T).%s() → ERROR: %v", d.r, method, err)
	} else {
		d.l.Printf("(%T).%s() → %#v", d.r, method, res)
	}
}

func (d *debugReader) Protocol() Protocol {
	return d.r.Protocol()
}

func (d *debugReader) Reader() io.Reader {
	return d.r.Reader()
}

func (d *debugReader) ReadBool() (bool, error) {
	v, err := d.r.ReadBool()
	d.log("ReadBool", v, err)
	return v, err
}

func (d *debugReader) ReadInt8() (int8, error) {
	v, err := d.r.ReadInt8()
	d.log("ReadInt8", v, err)
	return v, err
}

func (d *debugReader) ReadInt16() (int16, error) {
	v, err := d.r.ReadInt16()
	d.log("ReadInt16", v, err)
	return v, err
}

func (d *debugReader) ReadInt32() (int32, error) {
	v, err := d.r.ReadInt32()
	d.log("ReadInt32", v, err)
	return v, err
}

func (d *debugReader) ReadInt64() (int64, error) {
	v, err := d.r.ReadInt64()
	d.log("ReadInt64", v, err)
	return v, err
}

func (d *debugReader) ReadFloat64() (float64, error) {
	v, err := d.r.ReadFloat64()
	d.log("ReadFloat64", v, err)
	return v, err
}

func (d *debugReader) ReadBytes() ([]byte, error) {
	v, err := d.r.ReadBytes()
	d.log("ReadBytes", v, err)
	return v, err
}

func (d *debugReader) ReadString() (string, error) {
	v, err := d.r.ReadString()
	d.log("ReadString", v, err)
	return v, err
}

func (d *debugReader) ReadLength() (int, error) {
	v, err := d.r.ReadLength()
	d.log("ReadLength", v, err)
	return v, err
}

func (d *debugReader) ReadMessage() (Message, error) {
	v, err := d.r.ReadMessage()
	d.log("ReadMessage", v, err)
	return v, err
}

func (d *debugReader) ReadField() (Field, error) {
	v, err := d.r.ReadField()
	d.log("ReadField", v, err)
	return v, err
}

func (d *debugReader) ReadList() (List, error) {
	v, err := d.r.ReadList()
	d.log("ReadList", v, err)
	return v, err
}

func (d *debugReader) ReadSet() (Set, error) {
	v, err := d.r.ReadSet()
	d.log("ReadSet", v, err)
	return v, err
}

func (d *debugReader) ReadMap() (Map, error) {
	v, err := d.r.ReadMap()
	d.log("ReadMap", v, err)
	return v, err
}

type debugWriter struct {
	w Writer
	l *log.Logger
}

func (d *debugWriter) log(method string, arg any, err error) {
	if err != nil {
		d.l.Printf("(%T).%s(%#v) → ERROR: %v", d.w, method, arg, err)
	} else {
		d.l.Printf("(%T).%s(%#v)", d.w, method, arg)
	}
}

func (d *debugWriter) Protocol() Protocol {
	return d.w.Protocol()
}

func (d *debugWriter) Writer() io.Writer {
	return d.w.Writer()
}

func (d *debugWriter) WriteBool(v bool) error {
	err := d.w.WriteBool(v)
	d.log("WriteBool", v, err)
	return err
}

func (d *debugWriter) WriteInt8(v int8) error {
	err := d.w.WriteInt8(v)
	d.log("WriteInt8", v, err)
	return err
}

func (d *debugWriter) WriteInt16(v int16) error {
	err := d.w.WriteInt16(v)
	d.log("WriteInt16", v, err)
	return err
}

func (d *debugWriter) WriteInt32(v int32) error {
	err := d.w.WriteInt32(v)
	d.log("WriteInt32", v, err)
	return err
}

func (d *debugWriter) WriteInt64(v int64) error {
	err := d.w.WriteInt64(v)
	d.log("WriteInt64", v, err)
	return err
}

func (d *debugWriter) WriteFloat64(v float64) error {
	err := d.w.WriteFloat64(v)
	d.log("WriteFloat64", v, err)
	return err
}

func (d *debugWriter) WriteBytes(v []byte) error {
	err := d.w.WriteBytes(v)
	d.log("WriteBytes", v, err)
	return err
}

func (d *debugWriter) WriteString(v string) error {
	err := d.w.WriteString(v)
	d.log("WriteString", v, err)
	return err
}

func (d *debugWriter) WriteLength(n int) error {
	err := d.w.WriteLength(n)
	d.log("WriteLength", n, err)
	return err
}

func (d *debugWriter) WriteMessage(m Message) error {
	err := d.w.WriteMessage(m)
	d.log("WriteMessage", m, err)
	return err
}

func (d *debugWriter) WriteField(f Field) error {
	err := d.w.WriteField(f)
	d.log("WriteField", f, err)
	return err
}

func (d *debugWriter) WriteList(l List) error {
	err := d.w.WriteList(l)
	d.log("WriteList", l, err)
	return err
}

func (d *debugWriter) WriteSet(s Set) error {
	err := d.w.WriteSet(s)
	d.log("WriteSet", s, err)
	return err
}

func (d *debugWriter) WriteMap(m Map) error {
	err := d.w.WriteMap(m)
	d.log("WriteMap", m, err)
	return err
}
