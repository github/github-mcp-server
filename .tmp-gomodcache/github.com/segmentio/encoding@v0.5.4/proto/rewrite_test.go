package proto

import (
	"reflect"
	"testing"
)

func TestRewrite(t *testing.T) {
	type message struct {
		A int
		B float32
		C float64
		D string
		M *message
	}

	tests := []struct {
		scenario string
		in       message
		out      message
		rw       Rewriter
	}{
		{
			scenario: "identity",
			in:       message{A: 42, M: &message{A: 1}},
			out:      message{A: 42, M: &message{A: 1}},
			rw:       MessageRewriter(nil),
		},

		{
			scenario: "rewrite field 1",
			in:       message{A: 21},
			out:      message{A: 42},
			rw: MessageRewriter{
				1: FieldNumber(1).Int(42),
			},
		},

		{
			scenario: "rewrite field 2",
			in:       message{A: 21, B: 0.125},
			out:      message{A: 21, B: -1},
			rw: MessageRewriter{
				2: FieldNumber(2).Float32(-1),
			},
		},

		{
			scenario: "rewrite field 3",
			in:       message{A: 21, B: 0.125, C: 0.0},
			out:      message{A: 21, B: 0.125, C: 1.0},
			rw: MessageRewriter{
				3: FieldNumber(3).Float64(+1),
			},
		},

		{
			scenario: "rewrite field 4",
			in:       message{A: 21, B: 0.125, C: 1.0, D: "A"},
			out:      message{A: 21, B: 0.125, C: 1.0, D: "Hello World!"},
			rw: MessageRewriter{
				4: FieldNumber(4).String("Hello World!"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			b, err := Marshal(test.in)
			if err != nil {
				t.Fatal(err)
			}

			b, err = test.rw.Rewrite(nil, b)
			if err != nil {
				t.Fatal(err)
			}

			m := message{}
			if err := Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(m, test.out) {
				t.Errorf("messages mismatch:\nwant: %+v\ngot:  %+v", test.out, m)
			}
		})
	}
}

func TestParseRewriteTemplate(t *testing.T) {
	type submessage struct {
		Question string `protobuf:"bytes,1,opt,name=question,proto3"`
		Answer   string `protobuf:"bytes,2,opt,name=answer,proto3"`
	}

	type message struct {
		Field1 bool `protobuf:"varint,1,opt,name=field_1,proto3"`

		Field2 int   `protobuf:"varint,2,opt,name=field_2,proto3"`
		Field3 int32 `protobuf:"varint,3,opt,name=field_3,proto3"`
		Field4 int64 `protobuf:"varint,4,opt,name=field_4,proto3"`

		Field5 uint   `protobuf:"varint,5,opt,name=field_5,proto3"`
		Field6 uint32 `protobuf:"varint,6,opt,name=field_6,proto3"`
		Field7 uint64 `protobuf:"varint,7,opt,name=field_7,proto3"`

		Field8 int32 `protobuf:"zigzag32,8,opt,name=field_8,proto3"`
		Field9 int64 `protobuf:"zigzag64,9,opt,name=field_9,proto3"`

		Field10 float32 `protobuf:"fixed32,10,opt,name=field_10,proto3"`
		Field11 float64 `protobuf:"fixed64,11,opt,name=field_11,proto3"`

		Field12 string `protobuf:"bytes,12,opt,name=field_12,proto3"`
		Field13 []byte `protobuf:"bytes,13,opt,name=field_13,proto3"`

		Zero1 bool    `protobuf:"varint,21,opt,name=zero_1,proto3"`
		Zero2 int     `protobuf:"varint,22,opt,name=zero_2,proto3"`
		Zero3 int32   `protobuf:"varint,23,opt,name=zero_3,proto3"`
		Zero4 int64   `protobuf:"varint,24,opt,name=zero_4,proto3"`
		Zero5 uint    `protobuf:"varint,25,opt,name=zero_5,proto3"`
		Zero6 uint32  `protobuf:"varint,26,opt,name=zero_6,proto3"`
		Zero7 uint64  `protobuf:"varint,27,opt,name=zero_7,proto3"`
		Zero8 float32 `protobuf:"fixed32,28,opt,name=zero_8,proto3"`
		Zero9 float64 `protobuf:"fixed64,29,opt,name=zero_9,proto3"`

		Subfield    *submessage  `protobuf:"bytes,99,opt,name=subfield,proto3"`
		Submessages []submessage `protobuf:"bytes,100,rep,name=submessages,proto3"`

		Mapping map[string]int `protobuf:"bytes,200,opt,name=mapping,proto3"`
	}

	original := &message{
		Field1: false,

		Field2: -1,
		Field3: -2,
		Field4: -3,

		Field5: 1,
		Field6: 2,
		Field7: 3,

		Field8: -10,
		Field9: -11,

		Field10: 1.0,
		Field11: 2.0,

		Field12: "field 12",
		Field13: nil,

		Zero1: true,
		Zero2: 102,
		Zero3: 103,
		Zero4: 104,
		Zero5: 105,
		Zero6: 106,
		Zero7: 107,
		Zero8: 0.108,
		Zero9: 0.109,

		Subfield: &submessage{
			Answer: "Good!",
		},

		Submessages: []submessage{
			{Question: "Q1?", Answer: "A1"},
			{Question: "Q2?", Answer: "A2"},
			{Question: "Q3?", Answer: "A3"},
		},

		Mapping: map[string]int{
			"hello": 1,
			"world": 2,
		},
	}

	expected := &message{
		Field1: true,

		Field2: 2,
		Field3: 3,
		Field4: 4,

		Field5: 10,
		Field6: 11,
		Field7: 12,

		Field8: -21,
		Field9: -42,

		Field10: 0.25,
		Field11: 0.5,

		Field12: "Hello!",
		Field13: []byte("World!"),

		Subfield: &submessage{
			Question: "How are you?",
			Answer:   "Good!",
		},

		Submessages: []submessage{
			{Question: "Q1?", Answer: "A1"},
			{Question: "Q2?", Answer: "A2"},
			{Question: "Q3?", Answer: "Hello World!"},
		},

		Mapping: map[string]int{
			"answer": 42,
		},
	}

	rw, err := ParseRewriteTemplate(TypeOf(reflect.TypeOf(original)), []byte(`{
  "field_1": true,

  "field_2": 2,
  "field_3": 3,
  "field_4": 4,

  "field_5": 10,
  "field_6": 11,
  "field_7": 12,

  "field_8": -21,
  "field_9": -42,

  "field_10": 0.25,
  "field_11": 0.5,

  "field_12": "Hello!",
  "field_13": "World!",

  "zero_1": null,
  "zero_2": null,
  "zero_3": null,
  "zero_4": null,
  "zero_5": null,
  "zero_6": null,
  "zero_7": null,
  "zero_8": null,
  "zero_9": null,

  "subfield": {
    "question": "How are you?"
  },

  "submessages": [
    {
      "question": "Q1?",
      "answer": "A1"
    },
    {
      "question": "Q2?",
      "answer": "A2"
    },
    {
      "question": "Q3?",
      "answer": "Hello World!"
    }
  ],

  "mapping": {
    "answer": 42
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	b1, err := Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	b2, err := rw.Rewrite(nil, b1)
	if err != nil {
		t.Fatal(err)
	}

	found := &message{}
	if err := Unmarshal(b2, &found); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expected, found) {
		t.Error("messages mismatch after rewrite")
		t.Logf("want:\n%+v", expected)
		t.Logf("got:\n%+v", found)
	}
}

func TestParseRewriteRules(t *testing.T) {
	type submessage struct {
		Flags uint64 `protobuf:"varint,1,opt,name=flags,proto3"`
	}

	type message struct {
		Flags    uint64      `protobuf:"varint,2,opt,name=flags,proto3"`
		Subfield *submessage `protobuf:"bytes,99,opt,name=subfield,proto3"`
	}

	original := &message{
		Flags: 0b00000001,
		Subfield: &submessage{
			Flags: 0b00000010,
		},
	}

	expected := &message{
		Flags: 0b00000001 | 16,
		Subfield: &submessage{
			Flags: 0b00000010 | 32,
		},
	}

	rules := RewriterRules{
		"flags": BitOr[uint64]{},
		"subfield": RewriterRules{
			"flags": BitOr[uint64]{},
		},
	}

	rw, err := ParseRewriteTemplate(TypeOf(reflect.TypeOf(original)), []byte(`{
  "flags": 16,
  "subfield": {
    "flags": 32
  }
}`), rules)
	if err != nil {
		t.Fatal(err)
	}

	b1, err := Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	b2, err := rw.Rewrite(nil, b1)
	if err != nil {
		t.Fatal(err)
	}

	found := &message{}
	if err := Unmarshal(b2, &found); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expected, found) {
		t.Error("messages mismatch after rewrite")
		t.Logf("want:\n%+v", expected)
		t.Logf("got:\n%+v", found)
	}
}

func BenchmarkRewrite(b *testing.B) {
	type message struct {
		A int
		B float32
		C float64
		D string
	}

	in := message{A: 21, B: 0.125, D: "A"}
	rw := MessageRewriter{
		1: FieldNumber(1).Int(42),
		2: FieldNumber(2).Float32(-1),
		3: FieldNumber(3).Float64(+1),
		4: FieldNumber(4).String("Hello World!"),
	}

	p, err := Marshal(in)
	if err != nil {
		b.Fatal(err)
	}

	out := make([]byte, 0, 2*cap(p))

	for range b.N {
		out, err = rw.Rewrite(out[:0], p)
	}
}

func TestRewriteStructIdentity(t *testing.T) {
	type Node struct {
		Next               []Node   `protobuf:"bytes,1,rep,name=next,proto3" json:"next"`
		Name               string   `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
		Type               string   `protobuf:"bytes,3,opt,name=type,proto3" json:"type"`
		On                 uint32   `protobuf:"varint,4,opt,name=on,proto3,enum=op.Status" json:"on,omitempty"`
		Key                string   `protobuf:"bytes,5,opt,name=key,proto3" json:"key,omitempty"`
		Seed               uint64   `protobuf:"fixed64,6,opt,name=seed,proto3" json:"seed,omitempty"`
		ScheduleAfter      uint32   `protobuf:"varint,7,opt,name=schedule_after,json=scheduleAfter,proto3" json:"schedule_after,omitempty"`
		ExpireAfter        uint32   `protobuf:"varint,8,opt,name=expire_after,json=expireAfter,proto3" json:"expire_after,omitempty"`
		BackoffCoefficient uint32   `protobuf:"varint,9,opt,name=backoff_coefficient,json=backoffCoefficient,proto3" json:"backoff_coefficient,omitempty"`
		BackoffMinDelay    uint32   `protobuf:"varint,10,opt,name=backoff_min_delay,json=backoffMinDelay,proto3" json:"backoff_min_delay,omitempty"`
		BackoffMaxDelay    uint32   `protobuf:"varint,11,opt,name=backoff_max_delay,json=backoffMaxDelay,proto3" json:"backoff_max_delay,omitempty"`
		ExecutionTimeout   uint32   `protobuf:"varint,12,opt,name=execution_timeout,json=executionTimeout,proto3" json:"execution_timeout,omitempty"`
		NextLength         uint64   `protobuf:"varint,13,opt,name=next_length,json=nextLength,proto3" json:"next_length,omitempty"`
		BatchMaxBytes      uint32   `protobuf:"varint,14,opt,name=batch_max_bytes,json=batchMaxBytes,proto3" json:"batch_max_bytes,omitempty"`
		BatchMaxCount      uint32   `protobuf:"varint,15,opt,name=batch_max_count,json=batchMaxCount,proto3" json:"batch_max_count,omitempty"`
		BatchTimeout       uint32   `protobuf:"varint,16,opt,name=batch_timeout,json=batchTimeout,proto3" json:"batch_timeout,omitempty"`
		BatchKey           [16]byte `protobuf:"bytes,17,opt,name=batch_key,json=batchKey,proto3,customtype=U128" json:"batch_key"`
	}

	type Header struct {
		Flows         string `protobuf:"bytes,1,opt,name=flows,proto3" json:"flows,omitempty"`
		Root          Node   `protobuf:"bytes,2,opt,name=root,proto3" json:"root"`
		TraceContext  []byte `protobuf:"bytes,6,opt,name=trace_context,json=traceContext,proto3" json:"trace_context"`
		ContentLength int64  `protobuf:"varint,7,opt,name=content_length,json=contentLength,proto3" json:"content_length,omitempty"`
		ContentType   string `protobuf:"bytes,8,opt,name=content_type,json=contentType,proto3" json:"content_type"`
	}

	b := []byte{
		0xa, 0xd, 0x66, 0x6c, 0x6f, 0x77, 0x2d, 0x42, 0x3a, 0x66, 0x6c, 0x6f, 0x77, 0x2d, 0x30, 0x12,
		0x7a, 0xa, 0x30, 0x12, 0xc, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x64, 0x69, 0x73, 0x63, 0x61, 0x72,
		0x64, 0x1a, 0x0, 0x20, 0x2, 0x31, 0x69, 0xf2, 0xc9, 0xd0, 0xc1, 0x2f, 0xe0, 0x80, 0x68, 0x65,
		0x8a, 0x1, 0x10, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x12, 0x4, 0x74, 0x65, 0x73, 0x74, 0x1a, 0x4, 0x68, 0x74, 0x74, 0x70, 0x31,
		0xa, 0x7f, 0xf5, 0xf8, 0x13, 0x1d, 0xfb, 0x17, 0x38, 0xc1, 0xd, 0x40, 0xff, 0xdb, 0x1, 0x48,
		0xc6, 0xf, 0x50, 0xbd, 0x1b, 0x58, 0x9a, 0xbe, 0x2, 0x60, 0x88, 0x27, 0x68, 0x5d, 0x70, 0x80,
		0x80, 0x4, 0x78, 0xa, 0x80, 0x1, 0xd0, 0xf, 0x8a, 0x1, 0x10, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x32, 0x1c, 0xa, 0x10, 0x2a, 0x43,
		0x4c, 0xf3, 0x8c, 0x67, 0x48, 0x8f, 0xe, 0xca, 0xe8, 0x28, 0x96, 0x6c, 0x2b, 0xe4, 0x12,
		0x8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x38, 0x91, 0x66, 0x42, 0x18, 0x61, 0x70,
		0x70, 0x6c, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x6f, 0x63, 0x74, 0x65, 0x74, 0x2d,
		0x73, 0x74, 0x72, 0x65, 0x61, 0x6d,
	}

	m := Header{}
	if err := Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}

	r, err := ParseRewriteTemplate(TypeOf(reflect.TypeOf(m)), []byte(`{"root":{}}`))
	if err != nil {
		t.Fatal(err)
	}

	c, err := r.Rewrite(nil, b)
	if err != nil {
		t.Fatal(err)
	}

	x := Header{}
	if err := Unmarshal(c, &x); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(m, x) {
		t.Errorf("messages mismatch")
		t.Logf("want: %+v", m)
		t.Logf("got:  %+v", x)
	}
}
