package proto

import (
	"fmt"

	"github.com/segmentio/encoding/json"
)

// Rewriter is an interface implemented by types that support rewriting protobuf
// messages.
type Rewriter interface {
	// The function is expected to append the new content to the byte slice
	// passed as argument. If it wasn't able to perform the rewrite, it must
	// return a non-nil error.
	Rewrite(out, in []byte) ([]byte, error)
}

type identity struct{}

func (identity) Rewrite(out, in []byte) ([]byte, error) {
	return append(out, in...), nil
}

// MultiRewriter constructs a Rewriter which applies all rewriters passed as
// arguments.
func MultiRewriter(rewriters ...Rewriter) Rewriter {
	if len(rewriters) == 1 {
		return rewriters[0]
	}
	m := &multiRewriter{rewriters: make([]Rewriter, len(rewriters))}
	copy(m.rewriters, rewriters)
	return m
}

type multiRewriter struct {
	rewriters []Rewriter
}

func (m *multiRewriter) Rewrite(out, in []byte) ([]byte, error) {
	var err error

	for _, rw := range m.rewriters {
		if out, err = rw.Rewrite(out, in); err != nil {
			return out, err
		}
	}

	return out, nil
}

// RewriteFunc is a function type implementing the Rewriter interface.
type RewriteFunc func([]byte, []byte) ([]byte, error)

// Rewrite satisfies the Rewriter interface.
func (r RewriteFunc) Rewrite(out, in []byte) ([]byte, error) {
	return r(out, in)
}

// MessageRewriter maps field numbers to rewrite rules, satisfying the Rewriter
// interace to support composing rewrite rules.
type MessageRewriter []Rewriter

// Rewrite applies the rewrite rule matching f in r, satisfies the Rewriter
// interface.
func (r MessageRewriter) Rewrite(out, in []byte) ([]byte, error) {
	seen := make(fieldset, 4)

	if n := seen.len(); len(r) >= n {
		seen = makeFieldset(len(r) + 1)
	}

	for len(in) != 0 {
		f, t, v, m, err := Parse(in)
		if err != nil {
			return out, err
		}

		if i := int(f); i >= 0 && i < len(r) && r[i] != nil {
			if !seen.has(i) {
				seen.set(i)
				if out, err = r[i].Rewrite(out, v); err != nil {
					return out, err
				}
			}
		} else {
			out = Append(out, f, t, v)
		}

		in = m
	}

	for i, f := range r {
		if f != nil && !seen.has(i) {
			b, err := r[i].Rewrite(out, nil)
			if err != nil {
				return b, err
			}
			out = b
		}
	}

	return out, nil
}

type fieldset []uint64

func makeFieldset(n int) fieldset {
	if (n % 64) != 0 {
		n = (n + 1) / 64
	} else {
		n /= 64
	}
	return make(fieldset, n)
}

func (f fieldset) len() int {
	return len(f) * 64
}

func (f fieldset) has(i int) bool {
	x, y := f.index(i)
	return ((f[x] >> y) & 1) != 0
}

func (f fieldset) set(i int) {
	x, y := f.index(i)
	f[x] |= 1 << y
}

func (f fieldset) unset(i int) {
	x, y := f.index(i)
	f[x] &= ^(1 << y)
}

func (f fieldset) index(i int) (int, int) {
	return i / 64, i % 64
}

// ParseRewriteTemplate constructs a Rewriter for a protobuf type using the
// given json template to describe the rewrite rules.
//
// The json template contains a representation of the message that is used as the
// source values to overwrite in the protobuf targeted by the resulting rewriter.
//
// The rules are an optional set of RewriterRules that can provide alternative
// Rewriters from the default used for the field type. These rules are given the
// json.RawMessage bytes from the template, and they are expected to create a
// Rewriter to be applied against the target protobuf.
func ParseRewriteTemplate(typ Type, jsonTemplate []byte, rules ...RewriterRules) (Rewriter, error) {
	switch typ.Kind() {
	case Struct:
		return parseRewriteTemplateStruct(typ, 0, jsonTemplate, rules...)
	default:
		return nil, fmt.Errorf("cannot construct a rewrite template from a non-struct type %s", typ.Name())
	}
}

func parseRewriteTemplate(t Type, f FieldNumber, j json.RawMessage, rule any) (Rewriter, error) {
	if rwer, ok := rule.(Rewriterer); ok {
		return rwer.Rewriter(t, f, j)
	}

	switch t.Kind() {
	case Bool:
		return parseRewriteTemplateBool(t, f, j)
	case Int32:
		return parseRewriteTemplateInt32(t, f, j)
	case Int64:
		return parseRewriteTemplateInt64(t, f, j)
	case Sint32:
		return parseRewriteTemplateSint32(t, f, j)
	case Sint64:
		return parseRewriteTemplateSint64(t, f, j)
	case Uint32:
		return parseRewriteTemplateUint64(t, f, j)
	case Uint64:
		return parseRewriteTemplateUint64(t, f, j)
	case Fix32:
		return parseRewriteTemplateFix32(t, f, j)
	case Fix64:
		return parseRewriteTemplateFix64(t, f, j)
	case Sfix32:
		return parseRewriteTemplateSfix32(t, f, j)
	case Sfix64:
		return parseRewriteTemplateSfix64(t, f, j)
	case Float:
		return parseRewriteTemplateFloat(t, f, j)
	case Double:
		return parseRewriteTemplateDouble(t, f, j)
	case String:
		return parseRewriteTemplateString(t, f, j)
	case Bytes:
		return parseRewriteTemplateBytes(t, f, j)
	case Map:
		return parseRewriteTemplateMap(t, f, j)
	case Struct:
		sub, n, ok := [1]RewriterRules{}, 0, false
		if sub[0], ok = rule.(RewriterRules); ok {
			n = 1
		}
		return parseRewriteTemplateStruct(t, f, j, sub[:n]...)
	default:
		return nil, fmt.Errorf("cannot construct a rewriter from type %s", t.Name())
	}
}

func parseRewriteTemplateBool(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v bool
	err := json.Unmarshal(j, &v)
	if !v || err != nil {
		return nil, err
	}
	return f.Bool(v), nil
}

func parseRewriteTemplateInt32(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v int32
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Int32(v), nil
}

func parseRewriteTemplateInt64(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v int64
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Int64(v), nil
}

func parseRewriteTemplateSint32(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v int32
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Uint32(encodeZigZag32(v)), nil
}

func parseRewriteTemplateSint64(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v int64
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Uint64(encodeZigZag64(v)), nil
}

func parseRewriteTemplateUint32(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v uint32
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Uint32(v), nil
}

func parseRewriteTemplateUint64(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v uint64
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Uint64(v), nil
}

func parseRewriteTemplateFix32(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v uint32
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Fixed32(v), nil
}

func parseRewriteTemplateFix64(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v uint64
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Fixed64(v), nil
}

func parseRewriteTemplateSfix32(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v int32
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Fixed32(encodeZigZag32(v)), nil
}

func parseRewriteTemplateSfix64(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v int64
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Fixed64(encodeZigZag64(v)), nil
}

func parseRewriteTemplateFloat(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v float32
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Float32(v), nil
}

func parseRewriteTemplateDouble(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v float64
	err := json.Unmarshal(j, &v)
	if v == 0 || err != nil {
		return nil, err
	}
	return f.Float64(v), nil
}

func parseRewriteTemplateString(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v string
	err := json.Unmarshal(j, &v)
	if v == "" || err != nil {
		return nil, err
	}
	return f.String(v), nil
}

func parseRewriteTemplateBytes(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v string
	err := json.Unmarshal(j, &v)
	if v == "" || err != nil {
		return nil, err
	}
	return f.Bytes([]byte(v)), nil
}

func parseRewriteTemplateMap(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	st := &structType{
		name: t.Name(),
		fields: []Field{
			{Index: 0, Number: 1, Name: "key", Type: t.Key()},
			{Index: 1, Number: 2, Name: "value", Type: t.Elem()},
		},
		fieldsByName:   make(map[string]int),
		fieldsByNumber: make(map[FieldNumber]int),
	}

	for _, f := range st.fields {
		st.fieldsByName[f.Name] = f.Index
		st.fieldsByNumber[f.Number] = f.Index
	}

	template := map[string]json.RawMessage{}

	if err := json.Unmarshal(j, &template); err != nil {
		return nil, err
	}

	maplist := make([]json.RawMessage, 0, len(template))

	for key, value := range template {
		b, err := json.Marshal(struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}{
			Key:   key,
			Value: value,
		})
		if err != nil {
			return nil, err
		}
		maplist = append(maplist, b)
	}

	rewriters := make([]Rewriter, len(maplist))

	for i, b := range maplist {
		r, err := parseRewriteTemplateStruct(st, f, b)
		if err != nil {
			return nil, err
		}
		rewriters[i] = r
	}

	return MultiRewriter(rewriters...), nil
}

func parseRewriteTemplateStruct(t Type, f FieldNumber, j json.RawMessage, rules ...RewriterRules) (Rewriter, error) {
	template := map[string]json.RawMessage{}

	if err := json.Unmarshal(j, &template); err != nil {
		return nil, err
	}

	fieldsByName := map[string]Field{}

	for i := range t.NumField() {
		f := t.Field(i)
		fieldsByName[f.Name] = f
	}

	message := MessageRewriter{}
	rewriters := []Rewriter{}

	for k, v := range template {
		f, ok := fieldsByName[k]
		if !ok {
			return nil, fmt.Errorf("rewrite template contained an invalid field named %q", k)
		}

		var fields []json.RawMessage
		if f.Repeated {
			if err := json.Unmarshal(v, &fields); err != nil {
				return nil, err
			}
		} else {
			fields = []json.RawMessage{v}
		}

		var rule any
		for i := range rules {
			if r, ok := rules[i][f.Name]; ok {
				rule = r
				break
			}
		}

		rewriters = rewriters[:0]

		for _, v := range fields {
			rw, err := parseRewriteTemplate(f.Type, f.Number, v, rule)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			if rw != nil {
				rewriters = append(rewriters, rw)
			}
		}

		if cap(message) <= int(f.Number) {
			m := make(MessageRewriter, f.Number+1)
			copy(m, message)
			message = m
		}

		message[f.Number] = MultiRewriter(rewriters...)
	}

	if f != 0 {
		return &embddedRewriter{number: f, message: message}, nil
	}

	return message, nil
}

type embddedRewriter struct {
	number  FieldNumber
	message MessageRewriter
}

func (f *embddedRewriter) Rewrite(out, in []byte) ([]byte, error) {
	prefix := len(out)

	out, err := f.message.Rewrite(out, in)
	if err != nil {
		return nil, err
	}
	if len(out) == prefix {
		return out, nil
	}

	b := [24]byte{}
	n1, _ := encodeVarint(b[:], EncodeTag(f.number, Varlen))
	n2, _ := encodeVarint(b[n1:], uint64(len(out)-prefix))
	tagAndLen := n1 + n2

	out = append(out, b[:tagAndLen]...)
	copy(out[prefix+tagAndLen:], out[prefix:])
	copy(out[prefix:], b[:tagAndLen])
	return out, nil
}

// RewriterRules defines a set of rules for overriding the Rewriter used for any
// particular field. These maps may be nested for defining rules for struct members.
//
// For example:
//
//	rules := proto.RewriterRules {
//		"flags": proto.BitOr[uint64]{},
//		"nested": proto.RewriterRules {
//			"name": myCustomRewriter,
//		},
//	}
type RewriterRules map[string]any

// Rewriterer is the interface for producing a Rewriter for a given Type, FieldNumber
// and json.RawMessage. The JSON value is the JSON-encoded payload that should be
// decoded to produce the appropriate Rewriter. Implementations of the Rewriterer
// interface are added to the RewriterRules to specify the rules for performing
// custom rewrite logic.
type Rewriterer interface {
	Rewriter(Type, FieldNumber, json.RawMessage) (Rewriter, error)
}

// BitOr implments the Rewriterer interface for providing a bitwise-or rewrite
// logic for integers rather than replacing them. Instances of this type are
// zero-size, carrying only the generic type for creating the appropriate
// Rewriter when requested.
//
// Adding these to a RewriterRules looks like:
//
//	rules := proto.RewriterRules {
//		"flags": proto.BitOr[uint64]{},
//	}
//
// When used as a rule when rewriting from a template, the BitOr expects a JSON-
// encoded integer passed into the Rewriter method. This parsed integer is then
// used to perform a bitwise-or against the protobuf message that is being rewritten.
//
// The above example can then be used like:
//
//	template := []byte(`{"flags": 8}`) // n |= 0b1000
//	rw, err := proto.ParseRewriteTemplate(typ, template, rules)
type BitOr[T integer] struct{}

// integer is the contraint used by the BitOr Rewriterer and the bitOrRW Rewriter.
// Because these perform bitwise-or operations, the types must be integer-like.
type integer interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64
}

// Rewriter implements the Rewriterer interface. The JSON value provided to this
// method comes from the template used for rewriting. The returned Rewriter will use
// this JSON-encoded integer to perform a bitwise-or against the protobuf message
// that is being rewritten.
func (BitOr[T]) Rewriter(t Type, f FieldNumber, j json.RawMessage) (Rewriter, error) {
	var v T
	err := json.Unmarshal(j, &v)
	if err != nil {
		return nil, err
	}
	return BitOrRewriter(t, f, v)
}

// BitOrRewriter creates a bitwise-or Rewriter for a given field type and number.
// The mask is the value or'ed with values in the target protobuf.
func BitOrRewriter[T integer](t Type, f FieldNumber, mask T) (Rewriter, error) {
	switch t.Kind() {
	case Int32, Int64, Sint32, Sint64, Uint32, Uint64, Fix32, Fix64, Sfix32, Sfix64:
	default:
		return nil, fmt.Errorf("cannot construct a rewriter from type %s", t.Name())
	}
	return bitOrRW[T]{mask: mask, t: t, f: f}, nil
}

// bitOrRW is the Rewriter returned by the BitOr Rewriter method.
type bitOrRW[T integer] struct {
	mask T
	t    Type
	f    FieldNumber
}

// Rewrite implements the Rewriter interface performing a bitwise-or between the
// template value and the input value.
func (r bitOrRW[T]) Rewrite(out, in []byte) ([]byte, error) {
	var v T
	if err := Unmarshal(in, &v); err != nil {
		return nil, err
	}

	v |= r.mask

	switch r.t.Kind() {
	case Int32:
		return r.f.Int32(int32(v)).Rewrite(out, in)
	case Int64:
		return r.f.Int64(int64(v)).Rewrite(out, in)
	case Sint32:
		return r.f.Uint32(encodeZigZag32(int32(v))).Rewrite(out, in)
	case Sint64:
		return r.f.Uint64(encodeZigZag64(int64(v))).Rewrite(out, in)
	case Uint32, Uint64:
		return r.f.Uint64(uint64(v)).Rewrite(out, in)
	case Fix32:
		return r.f.Fixed32(uint32(v)).Rewrite(out, in)
	case Fix64:
		return r.f.Fixed64(uint64(v)).Rewrite(out, in)
	case Sfix32:
		return r.f.Fixed32(encodeZigZag32(int32(v))).Rewrite(out, in)
	case Sfix64:
		return r.f.Fixed64(encodeZigZag64(int64(v))).Rewrite(out, in)
	}

	panic("unreachable") // Kind is validated when creating instances
}
