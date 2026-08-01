package proto

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Kind is an enumeration representing the various data types supported by the
// protobuf language.
type Kind int

const (
	Bool Kind = iota
	Int32
	Int64
	Sint32
	Sint64
	Uint32
	Uint64
	Fix32
	Fix64
	Sfix32
	Sfix64
	Float
	Double
	String
	Bytes
	Map
	Struct
)

// Type is an interface similar to reflect.Type. Values implementing this
// interface represent high level protobuf types.
//
// Type values are safe to use concurrently from multiple goroutines.
//
// Types are comparable value.
type Type interface {
	// Returns a human-readable representation of the type.
	String() string

	// Returns the name of the type.
	Name() string

	// Kind returns the kind of protobuf values that are represented.
	Kind() Kind

	// When the Type represents a protobuf map, calling this method returns the
	// type of the map keys.
	//
	// If the Type is not a map type, the method panics.
	Key() Type

	// When the Type represents a protobuf map, calling this method returns the
	// type of the map values.
	//
	// If the Type is not a map type, the method panics.
	Elem() Type

	// Returns the protobuf wire type for the Type it is called on.
	WireType() WireType

	// Returns the number of fields in the protobuf message.
	//
	// If the Type does not represent a struct type, the method returns zero.
	NumField() int

	// Returns the Field at the given in Type.
	//
	// If the Type does not represent a struct type, the method panics.
	Field(int) Field

	// Returns the Field with the given name in Type.
	//
	// If the Type does not represent a struct type, or if the field does not
	// exist, the method panics.
	FieldByName(string) Field

	// Returns the Field with the given number in Type.
	//
	// If the Type does not represent a struct type, or if the field does not
	// exist, the method panics.
	FieldByNumber(FieldNumber) Field

	// For unsigned types, convert to their zig-zag form.
	//
	// The method uses the following table to perform the conversion:
	//
	//  base    | zig-zag
	//	--------+---------
	//	int32   | sint32
	//	int64   | sint64
	//	uint32  | sint32
	//	uint64  | sint64
	//	fixed32 | sfixed32
	//	fixed64 | sfixed64
	//
	// If the type cannot be converted to a zig-zag type, the method panics.
	ZigZag() Type
}

// TypeOf returns the protobuf type used to represent a go type.
//
// The function uses the following table to map Go types to Protobuf:
//
//	Go      | Protobuf
//	--------+---------
//	bool    | bool
//	int     | int64
//	int32   | int32
//	int64   | int64
//	uint    | uint64
//	uint32  | uint32
//	uint64  | uint64
//	float32 | float
//	float64 | double
//	string  | string
//	[]byte  | bytes
//	map     | map
//	struct  | message
//
// Pointer types are also supported and automatically dereferenced.
func TypeOf(t reflect.Type) Type {
	cache, _ := typesCache.Load().(map[reflect.Type]Type)
	if r, ok := cache[t]; ok {
		return r
	}

	typesMutex.Lock()
	defer typesMutex.Unlock()

	cache, _ = typesCache.Load().(map[reflect.Type]Type)
	if r, ok := cache[t]; ok {
		return r
	}

	seen := map[reflect.Type]Type{}
	r := typeOf(t, seen)

	newCache := make(map[reflect.Type]Type, len(cache)+len(seen))
	for t, r := range cache {
		newCache[t] = r
	}

	for t, r := range seen {
		if x, ok := newCache[t]; ok {
			r = x
		} else {
			newCache[t] = r
		}
	}

	if x, ok := newCache[t]; ok {
		r = x
	} else {
		newCache[t] = r
	}

	typesCache.Store(newCache)
	return r
}

func typeOf(t reflect.Type, seen map[reflect.Type]Type) Type {
	if r, ok := seen[t]; ok {
		return r
	}

	switch {
	case implements(t, messageType):
		return &opaqueMessageType{}
	case implements(t, customMessageType) && !implements(t, protoMessageType):
		return &primitiveTypes[Bytes]
	}

	switch t.Kind() {
	case reflect.Bool:
		return &primitiveTypes[Bool]
	case reflect.Int:
		return &primitiveTypes[Int64]
	case reflect.Int32:
		return &primitiveTypes[Int32]
	case reflect.Int64:
		return &primitiveTypes[Int64]
	case reflect.Uint:
		return &primitiveTypes[Uint64]
	case reflect.Uint32:
		return &primitiveTypes[Uint32]
	case reflect.Uint64:
		return &primitiveTypes[Uint64]
	case reflect.Float32:
		return &primitiveTypes[Float]
	case reflect.Float64:
		return &primitiveTypes[Double]
	case reflect.String:
		return &primitiveTypes[String]
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return &primitiveTypes[Bytes]
		}
	case reflect.Map:
		return mapTypeOf(t, seen)
	case reflect.Struct:
		return structTypeOf(t, seen)
	case reflect.Ptr:
		return typeOf(t.Elem(), seen)
	}

	panic(fmt.Errorf("cannot construct protobuf type from go value of type %s", t))
}

var (
	typesMutex sync.Mutex
	typesCache atomic.Value // map[reflect.Type]Type{}
)

type Field struct {
	Index    int
	Number   FieldNumber
	Name     string
	Type     Type
	Repeated bool
}

type primitiveType struct {
	name   string
	kind   Kind
	wire   WireType
	zigzag Kind
}

func (t *primitiveType) String() string {
	return t.name
}

func (t *primitiveType) Name() string {
	return t.name
}

func (t *primitiveType) Kind() Kind {
	return t.kind
}

func (t *primitiveType) Key() Type {
	panic(fmt.Errorf("proto.Type.Key: called on unsupported type: %s", t))
}

func (t *primitiveType) Elem() Type {
	panic(fmt.Errorf("proto.Type.Elem: called on unsupported type: %s", t))
}

func (t *primitiveType) WireType() WireType {
	return t.wire
}

func (t *primitiveType) NumField() int {
	return 0
}

func (t *primitiveType) Field(int) Field {
	panic(fmt.Errorf("proto.Type.Field: called on unsupported type: %s", t))
}

func (t *primitiveType) FieldByName(string) Field {
	panic(fmt.Errorf("proto.Type.FieldByName: called on unsupported type: %s", t))
}

func (t *primitiveType) FieldByNumber(FieldNumber) Field {
	panic(fmt.Errorf("proto.Type.FieldByNumber: called on unsupported type: %s", t))
}

func (t *primitiveType) ZigZag() Type {
	if t.zigzag == 0 {
		panic(fmt.Errorf("proto.Type.ZigZag: called on unsupported type: %s", t))
	}
	return &primitiveTypes[t.zigzag]
}

var primitiveTypes = [...]primitiveType{
	{name: "bool", kind: Bool, wire: Varint},
	{name: "int32", kind: Int32, wire: Varint, zigzag: Sint32},
	{name: "int64", kind: Int64, wire: Varint, zigzag: Sint64},
	{name: "sint32", kind: Sint32, wire: Varint},
	{name: "sint64", kind: Sint64, wire: Varint},
	{name: "uint32", kind: Uint32, wire: Varint, zigzag: Sint32},
	{name: "uint64", kind: Uint64, wire: Varint, zigzag: Sint64},
	{name: "fixed32", kind: Fix32, wire: Fixed32, zigzag: Sfix32},
	{name: "fixed64", kind: Fix64, wire: Fixed64, zigzag: Sfix64},
	{name: "sfixed32", kind: Sfix32, wire: Fixed32},
	{name: "sfixed64", kind: Sfix64, wire: Fixed64},
	{name: "float", kind: Float, wire: Fixed32},
	{name: "double", kind: Double, wire: Fixed64},
	{name: "string", kind: String, wire: Varlen},
	{name: "bytes", kind: Bytes, wire: Varlen},
}

func mapTypeOf(t reflect.Type, seen map[reflect.Type]Type) *mapType {
	mt := &mapType{}
	seen[t] = mt
	mt.key = typeOf(t.Key(), seen)
	mt.elem = typeOf(t.Elem(), seen)
	return mt
}

type mapType struct {
	key  Type
	elem Type
}

func (t *mapType) String() string {
	return fmt.Sprintf("map<%s, %s>", t.key.Name(), t.elem.Name())
}

func (t *mapType) Name() string {
	return t.String()
}

func (t *mapType) Kind() Kind {
	return Map
}

func (t *mapType) Key() Type {
	return t.key
}

func (t *mapType) Elem() Type {
	return t.elem
}

func (t *mapType) WireType() WireType {
	return Varlen
}

func (t *mapType) NumField() int {
	return 0
}

func (t *mapType) Field(int) Field {
	panic(fmt.Errorf("proto.Type.Field: called on unsupported type: %s", t))
}

func (t *mapType) FieldByName(string) Field {
	panic(fmt.Errorf("proto.Type.FieldByName: called on unsupported type: %s", t))
}

func (t *mapType) FieldByNumber(FieldNumber) Field {
	panic(fmt.Errorf("proto.Type.FieldByNumber: called on unsupported type: %s", t))
}

func (t *mapType) ZigZag() Type {
	panic(fmt.Errorf("proto.Type.ZigZag: called on unsupported type: %s", t))
}

func structTypeOf(t reflect.Type, seen map[reflect.Type]Type) *structType {
	st := &structType{
		name:           t.Name(),
		fieldsByName:   make(map[string]int),
		fieldsByNumber: make(map[FieldNumber]int),
	}

	seen[t] = st

	fieldNumber := FieldNumber(0)
	taggedFields := FieldNumber(0)

	for i := range t.NumField() {
		f := t.Field(i)

		if f.PkgPath != "" {
			continue // unexported
		}

		repeated := false
		if f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() != reflect.Uint8 {
			repeated = true
			f.Type = f.Type.Elem() // for typeOf
		}

		fieldName := f.Name
		fieldType := typeOf(f.Type, seen)

		if tag, ok := f.Tag.Lookup("protobuf"); ok {
			if fieldNumber != taggedFields {
				panic("conflicting use of struct tag and naked fields")
			}
			t, err := parseStructTag(tag)
			if err != nil {
				panic(err)
			}

			fieldName = t.name
			fieldNumber = t.fieldNumber
			taggedFields = t.fieldNumber
			// Because maps are represented as repeated varlen fields on the
			// wire, the generated protobuf code sets the `rep` attribute on
			// the struct fields.
			repeated = t.repeated && f.Type.Kind() != reflect.Map

			if t.zigzag {
				fieldType = fieldType.ZigZag()
			}
		} else if fieldNumber == 0 && len(st.fields) != 0 {
			panic("conflicting use of struct tag and naked fields")
		} else {
			fieldNumber++
		}

		index := len(st.fields)
		st.fields = append(st.fields, Field{
			Index:    index,
			Number:   fieldNumber,
			Name:     fieldName,
			Type:     fieldType,
			Repeated: repeated,
		})
		st.fieldsByName[fieldName] = index
		st.fieldsByNumber[fieldNumber] = index
	}

	return st
}

type structType struct {
	name           string
	fields         []Field
	fieldsByName   map[string]int
	fieldsByNumber map[FieldNumber]int
}

func (t *structType) String() string {
	s := strings.Builder{}
	s.WriteString("message ")

	if t.name != "" {
		s.WriteString(t.name)
		s.WriteString(" ")
	}

	s.WriteString("{")

	for _, f := range t.fields {
		s.WriteString("\n  ")

		if f.Repeated {
			s.WriteString("repeated ")
		} else {
		}

		s.WriteString(f.Type.Name())
		s.WriteString(" ")
		s.WriteString(f.Name)
		s.WriteString(" = ")
		s.WriteString(strconv.Itoa(int(f.Number)))
		s.WriteString(";")
	}

	if len(t.fields) == 0 {
		s.WriteString("}")
	} else {
		s.WriteString("\n}")
	}

	return s.String()
}

func (t *structType) Name() string {
	return t.name
}

func (t *structType) Kind() Kind {
	return Struct
}

func (t *structType) Key() Type {
	panic(fmt.Errorf("proto.Type.Key: called on unsupported type: %s", t.name))
}

func (t *structType) Elem() Type {
	panic(fmt.Errorf("proto.Type.Elem: called on unsupported type: %s", t.name))
}

func (t *structType) WireType() WireType {
	return Varlen
}

func (t *structType) NumField() int {
	return len(t.fields)
}

func (t *structType) Field(index int) Field {
	if index >= 0 && index < len(t.fields) {
		return t.fields[index]
	}
	panic(fmt.Errorf("proto.Type.Field: protobuf message field out of bounds: %d/%d", index, len(t.fields)))
}

func (t *structType) FieldByName(name string) Field {
	i, ok := t.fieldsByName[name]
	if ok {
		return t.fields[i]
	}
	panic(fmt.Errorf("proto.Type.FieldByName: protobuf message has not field named %q", name))
}

func (t *structType) FieldByNumber(number FieldNumber) Field {
	i, ok := t.fieldsByNumber[number]
	if ok {
		return t.fields[i]
	}
	panic(fmt.Errorf("proto.Type.FieldByNumber: protobuf message has no field number %d", number))
}

func (t *structType) ZigZag() Type {
	panic(fmt.Errorf("proto.Type.ZigZag: called on unsupported type: %s", t.name))
}

type structTag struct {
	name        string
	enum        string
	json        string
	version     int
	wireType    WireType
	fieldNumber FieldNumber
	extensions  map[string]string
	repeated    bool
	zigzag      bool
}

func parseStructTag(tag string) (structTag, error) {
	t := structTag{
		version:    2,
		extensions: make(map[string]string),
	}

	for i, f := range splitFields(tag) {
		switch i {
		case 0:
			switch f {
			case "varint":
				t.wireType = Varint
			case "bytes":
				t.wireType = Varlen
			case "fixed32":
				t.wireType = Fixed32
			case "fixed64":
				t.wireType = Fixed64
			case "zigzag32":
				t.wireType = Varint
				t.zigzag = true
			case "zigzag64":
				t.wireType = Varint
				t.zigzag = true
			default:
				return t, fmt.Errorf("unsupported wire type in struct tag %q: %s", tag, f)
			}

		case 1:
			n, err := strconv.Atoi(f)
			if err != nil {
				return t, fmt.Errorf("unsupported field number in struct tag %q: %w", tag, err)
			}
			t.fieldNumber = FieldNumber(n)

		case 2:
			switch f {
			case "opt":
				// not sure what this is for
			case "rep":
				t.repeated = true
			default:
				return t, fmt.Errorf("unsupported field option in struct tag %q: %s", tag, f)
			}

		default:
			name, value := splitNameValue(f)
			switch name {
			case "name":
				t.name = value
			case "enum":
				t.enum = value
			case "json":
				t.json = value
			case "proto3":
				t.version = 3
			default:
				t.extensions[name] = value
			}
		}
	}

	return t, nil
}

func splitFields(s string) []string {
	return strings.Split(s, ",")
}

func splitNameValue(s string) (name, value string) {
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return strings.TrimSpace(s), ""
	} else {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
}

type opaqueMessageType struct{}

func (t *opaqueMessageType) String() string {
	return "bytes"
}

func (t *opaqueMessageType) Name() string {
	return "bytes"
}

func (t *opaqueMessageType) Kind() Kind {
	return Struct
}

func (t *opaqueMessageType) Key() Type {
	panic(fmt.Errorf("proto.Type.Key: called on unsupported type: %s", t))
}

func (t *opaqueMessageType) Elem() Type {
	panic(fmt.Errorf("proto.Type.Elem: called on unsupported type: %s", t))
}

func (t *opaqueMessageType) WireType() WireType {
	return Varlen
}

func (t *opaqueMessageType) NumField() int {
	return 0
}

func (t *opaqueMessageType) Field(int) Field {
	panic(fmt.Errorf("proto.Type.Field: called on unsupported type: %s", t))
}

func (t *opaqueMessageType) FieldByName(string) Field {
	panic(fmt.Errorf("proto.Type.FieldByName: called on unsupported type: %s", t))
}

func (t *opaqueMessageType) FieldByNumber(FieldNumber) Field {
	panic(fmt.Errorf("proto.Type.FieldByNumber: called on unsupported type: %s", t))
}

func (t *opaqueMessageType) ZigZag() Type {
	panic(fmt.Errorf("proto.Type.ZigZag: called on unsupported type: %s", t))
}
