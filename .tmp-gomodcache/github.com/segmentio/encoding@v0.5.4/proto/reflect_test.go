package proto

import (
	"fmt"
	"reflect"
	"testing"
)

type RecursiveMessage struct {
	Next *RecursiveMessage `protobuf:"bytes,1,opt,name=next,proto3"`
}

func TestTypeOf(t *testing.T) {
	tests := []struct {
		value any
		proto string
	}{
		// primitive types
		{value: true, proto: "bool"},
		{value: int(1), proto: "int64"},
		{value: int32(1), proto: "int32"},
		{value: int64(1), proto: "int64"},
		{value: uint(1), proto: "uint64"},
		{value: uint32(1), proto: "uint32"},
		{value: uint64(1), proto: "uint64"},
		{value: float32(1), proto: "float"},
		{value: float64(1), proto: "double"},
		{value: "hello", proto: "string"},
		{value: []byte("A"), proto: "bytes"},

		// map types
		{value: map[int]string{}, proto: "map<int64, string>"},

		// struct types
		{
			value: struct{}{},
			proto: `message {}`,
		},

		{
			value: struct{ A int }{},
			proto: `message {
  int64 A = 1;
}`,
		},

		{
			value: struct {
				A int    `protobuf:"varint,1,opt,name=hello,proto3"`
				B string `protobuf:"bytes,3,rep,name=world,proto3"`
			}{},
			proto: `message {
  int64 hello = 1;
  repeated string world = 3;
}`,
		},

		{
			value: RecursiveMessage{},
			proto: `message RecursiveMessage {
  RecursiveMessage next = 1;
}`,
		},

		{
			value: struct {
				M RawMessage
			}{},
			proto: `message {
  bytes M = 1;
}`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%T", test.value), func(t *testing.T) {
			typ := TypeOf(reflect.TypeOf(test.value))
			str := typ.String()

			if str != test.proto {
				t.Error("protobuf representation mismatch")
				t.Log("want:", test.proto)
				t.Log("got: ", str)
			}
		})
	}
}

func TestTypesAreEqual(t *testing.T) {
	if TypeOf(reflect.TypeOf(true)) != TypeOf(reflect.TypeOf(false)) {
		t.Error("type of true is not equal to type of false")
	}
}

func TestTypesAreNotEqual(t *testing.T) {
	if TypeOf(reflect.TypeOf(false)) == TypeOf(reflect.TypeOf(0)) {
		t.Error("type of bool equal type of int")
	}
}

func TestParseStructTag(t *testing.T) {
	tests := []struct {
		str string
		tag structTag
	}{
		{
			str: `bytes,1,rep,name=next,proto3`,
			tag: structTag{
				name:        "next",
				version:     3,
				wireType:    Varlen,
				fieldNumber: 1,
				extensions:  map[string]string{},
				repeated:    true,
			},
		},

		{
			str: `bytes,5,opt,name=key,proto3`,
			tag: structTag{
				name:        "key",
				version:     3,
				wireType:    Varlen,
				fieldNumber: 5,
				extensions:  map[string]string{},
			},
		},

		{
			str: `fixed64,6,opt,name=seed,proto3`,
			tag: structTag{
				name:        "seed",
				version:     3,
				wireType:    Fixed64,
				fieldNumber: 6,
				extensions:  map[string]string{},
			},
		},

		{
			str: `varint,8,opt,name=expire_after,json=expireAfter,proto3`,
			tag: structTag{
				name:        "expire_after",
				json:        "expireAfter",
				version:     3,
				wireType:    Varint,
				fieldNumber: 8,
				extensions:  map[string]string{},
			},
		},

		{
			str: `bytes,17,opt,name=batch_key,json=batchKey,proto3,customtype=U128`,
			tag: structTag{
				name:        "batch_key",
				json:        "batchKey",
				version:     3,
				wireType:    Varlen,
				fieldNumber: 17,
				extensions: map[string]string{
					"customtype": "U128",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.str, func(t *testing.T) {
			tag, err := parseStructTag(test.str)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tag, test.tag) {
				t.Errorf("struct tag mismatch\nwant: %+v\ngot: %+v", test.tag, tag)
			}
		})
	}
}
