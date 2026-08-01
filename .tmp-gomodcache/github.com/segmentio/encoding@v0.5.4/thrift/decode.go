package thrift

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sync/atomic"
)

// Unmarshal deserializes the thrift data from b to v using to the protocol p.
//
// The function errors if the data in b does not match the type of v.
//
// The function panics if v cannot be converted to a thrift representation.
//
// As an optimization, the value passed in v may be reused across multiple calls
// to Unmarshal, allowing the function to reuse objects referenced by pointer
// fields of struct values. When reusing objects, the application is responsible
// for resetting the state of v before calling Unmarshal again.
func Unmarshal(p Protocol, b []byte, v any) error {
	br := bytes.NewReader(b)
	pr := p.NewReader(br)

	if err := NewDecoder(pr).Decode(v); err != nil {
		return err
	}

	if n := br.Len(); n != 0 {
		return fmt.Errorf("unexpected trailing bytes at the end of thrift input: %d", n)
	}

	return nil
}

type Decoder struct {
	r Reader
	f flags
}

func NewDecoder(r Reader) *Decoder {
	return &Decoder{r: r, f: decoderFlags(r)}
}

func (d *Decoder) Decode(v any) error {
	t := reflect.TypeOf(v)
	p := reflect.ValueOf(v)

	if t.Kind() != reflect.Ptr {
		panic("thrift.(*Decoder).Decode: expected pointer type but got " + t.String())
	}

	t = t.Elem()
	p = p.Elem()

	cache, _ := decoderCache.Load().(map[typeID]decodeFunc)
	decode := cache[makeTypeID(t)]

	if decode == nil {
		decode = decodeFuncOf(t, make(decodeFuncCache))

		newCache := make(map[typeID]decodeFunc, len(cache)+1)
		newCache[makeTypeID(t)] = decode
		for k, v := range cache {
			newCache[k] = v
		}

		decoderCache.Store(newCache)
	}

	return decode(d.r, p, d.f)
}

func (d *Decoder) Reset(r Reader) {
	d.r = r
	d.f = d.f.without(protocolFlags).with(decoderFlags(r))
}

func (d *Decoder) SetStrict(enabled bool) {
	if enabled {
		d.f = d.f.with(strict)
	} else {
		d.f = d.f.without(strict)
	}
}

func decoderFlags(r Reader) flags {
	return flags(r.Protocol().Features() << featuresBitOffset)
}

var decoderCache atomic.Value // map[typeID]decodeFunc

type decodeFunc func(Reader, reflect.Value, flags) error

type decodeFuncCache map[reflect.Type]decodeFunc

func decodeFuncOf(t reflect.Type, seen decodeFuncCache) decodeFunc {
	f := seen[t]
	if f != nil {
		return f
	}
	switch t.Kind() {
	case reflect.Bool:
		f = decodeBool
	case reflect.Int8:
		f = decodeInt8
	case reflect.Int16:
		f = decodeInt16
	case reflect.Int32:
		f = decodeInt32
	case reflect.Int64, reflect.Int:
		f = decodeInt64
	case reflect.Float32, reflect.Float64:
		f = decodeFloat64
	case reflect.String:
		f = decodeString
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 { // []byte
			f = decodeBytes
		} else {
			f = decodeFuncSliceOf(t, seen)
		}
	case reflect.Map:
		f = decodeFuncMapOf(t, seen)
	case reflect.Struct:
		f = decodeFuncStructOf(t, seen)
	case reflect.Ptr:
		f = decodeFuncPtrOf(t, seen)
	default:
		panic("type cannot be decoded in thrift: " + t.String())
	}
	seen[t] = f
	return f
}

func decodeBool(r Reader, v reflect.Value, _ flags) error {
	b, err := r.ReadBool()
	if err != nil {
		return err
	}
	v.SetBool(b)
	return nil
}

func decodeInt8(r Reader, v reflect.Value, _ flags) error {
	i, err := r.ReadInt8()
	if err != nil {
		return err
	}
	v.SetInt(int64(i))
	return nil
}

func decodeInt16(r Reader, v reflect.Value, _ flags) error {
	i, err := r.ReadInt16()
	if err != nil {
		return err
	}
	v.SetInt(int64(i))
	return nil
}

func decodeInt32(r Reader, v reflect.Value, _ flags) error {
	i, err := r.ReadInt32()
	if err != nil {
		return err
	}
	v.SetInt(int64(i))
	return nil
}

func decodeInt64(r Reader, v reflect.Value, _ flags) error {
	i, err := r.ReadInt64()
	if err != nil {
		return err
	}
	v.SetInt(int64(i))
	return nil
}

func decodeFloat64(r Reader, v reflect.Value, _ flags) error {
	f, err := r.ReadFloat64()
	if err != nil {
		return err
	}
	v.SetFloat(f)
	return nil
}

func decodeString(r Reader, v reflect.Value, _ flags) error {
	s, err := r.ReadString()
	if err != nil {
		return err
	}
	v.SetString(s)
	return nil
}

func decodeBytes(r Reader, v reflect.Value, _ flags) error {
	b, err := r.ReadBytes()
	if err != nil {
		return err
	}
	v.SetBytes(b)
	return nil
}

func decodeFuncSliceOf(t reflect.Type, seen decodeFuncCache) decodeFunc {
	elem := t.Elem()
	typ := TypeOf(elem)
	dec := decodeFuncOf(elem, seen)

	return func(r Reader, v reflect.Value, flags flags) error {
		l, err := r.ReadList()
		if err != nil {
			return err
		}

		// Sometimes the list type is set to TRUE when the list contains only
		// TRUE values. Thrift does not seem to optimize the encoding by
		// omitting the boolean values that are known to all be TRUE, we still
		// need to decode them.
		switch l.Type {
		case TRUE:
			l.Type = BOOL
		}

		// TODO: implement type conversions?
		if typ != l.Type {
			if flags.have(strict) {
				return &TypeMismatch{item: "list item", Expect: typ, Found: l.Type}
			}
			return nil
		}

		v.Set(reflect.MakeSlice(t, int(l.Size), int(l.Size)))
		flags = flags.only(decodeFlags)

		for i := range int(l.Size) {
			if err := dec(r, v.Index(i), flags); err != nil {
				return with(dontExpectEOF(err), &decodeErrorList{cause: l, index: i})
			}
		}

		return nil
	}
}

func decodeFuncMapOf(t reflect.Type, seen decodeFuncCache) decodeFunc {
	key, elem := t.Key(), t.Elem()
	if elem.Size() == 0 { // map[?]struct{}
		return decodeFuncMapAsSetOf(t, seen)
	}

	mapType := reflect.MapOf(key, elem)
	keyZero := reflect.Zero(key)
	elemZero := reflect.Zero(elem)
	keyType := TypeOf(key)
	elemType := TypeOf(elem)
	decodeKey := decodeFuncOf(key, seen)
	decodeElem := decodeFuncOf(elem, seen)

	return func(r Reader, v reflect.Value, flags flags) error {
		m, err := r.ReadMap()
		if err != nil {
			return err
		}

		v.Set(reflect.MakeMapWithSize(mapType, int(m.Size)))

		if m.Size == 0 { // empty map
			return nil
		}

		// TODO: implement type conversions?
		if keyType != m.Key {
			if flags.have(strict) {
				return &TypeMismatch{item: "map key", Expect: keyType, Found: m.Key}
			}
			return nil
		}

		if elemType != m.Value {
			if flags.have(strict) {
				return &TypeMismatch{item: "map value", Expect: elemType, Found: m.Value}
			}
			return nil
		}

		tmpKey := reflect.New(key).Elem()
		tmpElem := reflect.New(elem).Elem()
		flags = flags.only(decodeFlags)

		for i := range int(m.Size) {
			if err := decodeKey(r, tmpKey, flags); err != nil {
				return with(dontExpectEOF(err), &decodeErrorMap{cause: m, index: i})
			}
			if err := decodeElem(r, tmpElem, flags); err != nil {
				return with(dontExpectEOF(err), &decodeErrorMap{cause: m, index: i})
			}
			v.SetMapIndex(tmpKey, tmpElem)
			tmpKey.Set(keyZero)
			tmpElem.Set(elemZero)
		}

		return nil
	}
}

func decodeFuncMapAsSetOf(t reflect.Type, seen decodeFuncCache) decodeFunc {
	key, elem := t.Key(), t.Elem()
	keyZero := reflect.Zero(key)
	elemZero := reflect.Zero(elem)
	typ := TypeOf(key)
	dec := decodeFuncOf(key, seen)

	return func(r Reader, v reflect.Value, flags flags) error {
		s, err := r.ReadSet()
		if err != nil {
			return err
		}

		// See decodeFuncSliceOf for details about why this type conversion
		// needs to be done.
		switch s.Type {
		case TRUE:
			s.Type = BOOL
		}

		v.Set(reflect.MakeMapWithSize(t, int(s.Size)))

		if s.Size == 0 {
			return nil
		}

		// TODO: implement type conversions?
		if typ != s.Type {
			if flags.have(strict) {
				return &TypeMismatch{item: "list item", Expect: typ, Found: s.Type}
			}
			return nil
		}

		tmp := reflect.New(key).Elem()
		flags = flags.only(decodeFlags)

		for i := range int(s.Size) {
			if err := dec(r, tmp, flags); err != nil {
				return with(dontExpectEOF(err), &decodeErrorSet{cause: s, index: i})
			}
			v.SetMapIndex(tmp, elemZero)
			tmp.Set(keyZero)
		}

		return nil
	}
}

type structDecoder struct {
	fields   []structDecoderField
	union    []int
	minID    int16
	zero     reflect.Value
	required []uint64
}

func (dec *structDecoder) decode(r Reader, v reflect.Value, flags flags) error {
	flags = flags.only(decodeFlags)
	coalesceBoolFields := flags.have(coalesceBoolFields)

	lastField := reflect.Value{}
	union := len(dec.union) > 0
	seen := make([]uint64, 1)
	if len(dec.required) > len(seen) {
		seen = make([]uint64, len(dec.required))
	}

	err := readStruct(r, func(r Reader, f Field) error {
		i := int(f.ID) - int(dec.minID)
		if i < 0 || i >= len(dec.fields) || dec.fields[i].decode == nil {
			return skipField(r, f)
		}
		field := &dec.fields[i]
		seen[i/64] |= 1 << (i % 64)

		// TODO: implement type conversions?
		if f.Type != field.typ && (f.Type != TRUE || field.typ != BOOL) {
			if flags.have(strict) {
				return &TypeMismatch{item: "field value", Expect: field.typ, Found: f.Type}
			}
			return nil
		}

		x := v
		for _, i := range field.index {
			if x.Kind() == reflect.Ptr {
				x = x.Elem()
			}
			if x = x.Field(i); x.Kind() == reflect.Ptr {
				if x.IsNil() {
					x.Set(reflect.New(x.Type().Elem()))
				}
			}
		}

		if union {
			v.Set(dec.zero)
		}

		lastField = x

		if coalesceBoolFields && (f.Type == TRUE || f.Type == FALSE) {
			for x.Kind() == reflect.Ptr {
				if x.IsNil() {
					x.Set(reflect.New(x.Type().Elem()))
				}
				x = x.Elem()
			}
			x.SetBool(f.Type == TRUE)
			return nil
		}

		return field.decode(r, x, flags.with(field.flags))
	})
	if err != nil {
		return err
	}

	for i, required := range dec.required {
		if mask := required & seen[i]; mask != required {
			i *= 64
			for (mask & 1) != 0 {
				mask >>= 1
				i++
			}
			field := &dec.fields[i]
			return &MissingField{Field: Field{ID: field.id, Type: field.typ}}
		}
	}

	if union && lastField.IsValid() {
		v.FieldByIndex(dec.union).Set(lastField.Addr())
	}

	return nil
}

type structDecoderField struct {
	index  []int
	id     int16
	flags  flags
	typ    Type
	decode decodeFunc
}

func decodeFuncStructOf(t reflect.Type, seen decodeFuncCache) decodeFunc {
	dec := &structDecoder{
		zero: reflect.Zero(t),
	}
	decode := dec.decode
	seen[t] = decode

	fields := make([]structDecoderField, 0, t.NumField())
	forEachStructField(t, nil, func(f structField) {
		if f.flags.have(union) {
			dec.union = f.index
		} else {
			fields = append(fields, structDecoderField{
				index:  f.index,
				id:     f.id,
				flags:  f.flags,
				typ:    TypeOf(f.typ),
				decode: decodeFuncStructFieldOf(f, seen),
			})
		}
	})

	minID := int16(0)
	maxID := int16(0)

	for _, f := range fields {
		if f.id < minID || minID == 0 {
			minID = f.id
		}
		if f.id > maxID {
			maxID = f.id
		}
	}

	dec.fields = make([]structDecoderField, (maxID-minID)+1)
	dec.minID = minID
	dec.required = make([]uint64, len(fields)/64+1)

	for _, f := range fields {
		i := f.id - minID
		p := dec.fields[i]
		if p.decode != nil {
			panic(fmt.Errorf("thrift struct field id %d is present multiple times in %s with types %s and %s", f.id, t, p.typ, f.typ))
		}
		dec.fields[i] = f
		if f.flags.have(required) {
			dec.required[i/64] |= 1 << (i % 64)
		}
	}

	return decode
}

func decodeFuncStructFieldOf(f structField, seen decodeFuncCache) decodeFunc {
	if f.flags.have(enum) {
		switch f.typ.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return decodeInt32
		}
	}
	return decodeFuncOf(f.typ, seen)
}

func decodeFuncPtrOf(t reflect.Type, seen decodeFuncCache) decodeFunc {
	elem := t.Elem()
	decode := decodeFuncOf(t.Elem(), seen)
	return func(r Reader, v reflect.Value, f flags) error {
		if v.IsNil() {
			v.Set(reflect.New(elem))
		}
		return decode(r, v.Elem(), f)
	}
}

func readBinary(r Reader, f func(io.Reader) error) error {
	n, err := r.ReadLength()
	if err != nil {
		return err
	}
	return dontExpectEOF(f(io.LimitReader(r.Reader(), int64(n))))
}

func readList(r Reader, f func(Reader, Type) error) error {
	l, err := r.ReadList()
	if err != nil {
		return err
	}

	for i := range int(l.Size) {
		if err := f(r, l.Type); err != nil {
			return with(dontExpectEOF(err), &decodeErrorList{cause: l, index: i})
		}
	}

	return nil
}

func readSet(r Reader, f func(Reader, Type) error) error {
	s, err := r.ReadSet()
	if err != nil {
		return err
	}

	for i := range int(s.Size) {
		if err := f(r, s.Type); err != nil {
			return with(dontExpectEOF(err), &decodeErrorSet{cause: s, index: i})
		}
	}

	return nil
}

func readMap(r Reader, f func(Reader, Type, Type) error) error {
	m, err := r.ReadMap()
	if err != nil {
		return err
	}

	for i := range int(m.Size) {
		if err := f(r, m.Key, m.Value); err != nil {
			return with(dontExpectEOF(err), &decodeErrorMap{cause: m, index: i})
		}
	}

	return nil
}

func readStruct(r Reader, f func(Reader, Field) error) error {
	lastFieldID := int16(0)
	numFields := 0

	for {
		x, err := r.ReadField()
		if err != nil {
			if numFields > 0 {
				err = dontExpectEOF(err)
			}
			return err
		}

		if x.Type == STOP {
			return nil
		}

		if x.Delta {
			x.ID += lastFieldID
			x.Delta = false
		}

		if err := f(r, x); err != nil {
			return with(dontExpectEOF(err), &decodeErrorField{cause: x})
		}

		lastFieldID = x.ID
		numFields++
	}
}

func skip(r Reader, t Type) error {
	var err error
	switch t {
	case TRUE, FALSE:
		_, err = r.ReadBool()
	case I8:
		_, err = r.ReadInt8()
	case I16:
		_, err = r.ReadInt16()
	case I32:
		_, err = r.ReadInt32()
	case I64:
		_, err = r.ReadInt64()
	case DOUBLE:
		_, err = r.ReadFloat64()
	case BINARY:
		err = skipBinary(r)
	case LIST:
		err = skipList(r)
	case SET:
		err = skipSet(r)
	case MAP:
		err = skipMap(r)
	case STRUCT:
		err = skipStruct(r)
	default:
		return fmt.Errorf("skipping unsupported thrift type %d", t)
	}
	return err
}

func skipBinary(r Reader) error {
	n, err := r.ReadLength()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	switch x := r.Reader().(type) {
	case *bufio.Reader:
		_, err = x.Discard(int(n))
	default:
		_, err = io.CopyN(io.Discard, x, int64(n))
	}
	return dontExpectEOF(err)
}

func skipList(r Reader) error {
	return readList(r, skip)
}

func skipSet(r Reader) error {
	return readSet(r, skip)
}

func skipMap(r Reader) error {
	return readMap(r, func(r Reader, k, v Type) error {
		if err := skip(r, k); err != nil {
			return dontExpectEOF(err)
		}
		if err := skip(r, v); err != nil {
			return dontExpectEOF(err)
		}
		return nil
	})
}

func skipStruct(r Reader) error {
	return readStruct(r, skipField)
}

func skipField(r Reader, f Field) error {
	return skip(r, f.Type)
}
