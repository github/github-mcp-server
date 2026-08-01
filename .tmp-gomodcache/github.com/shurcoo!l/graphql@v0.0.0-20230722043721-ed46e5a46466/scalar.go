package graphql

// Note: These custom types are meant to be used in queries for now.
// But the plan is to switch to using native Go types (string, int, bool, time.Time, etc.).
// See https://github.com/shurcooL/githubv4/issues/9 for details.
//
// These custom types currently provide documentation, and their use
// is required for sending outbound queries. However, native Go types
// can be used for unmarshaling. Once https://github.com/shurcooL/githubv4/issues/9
// is resolved, native Go types can completely replace these.

type (
	// Boolean represents true or false values.
	Boolean bool

	// Float represents signed double-precision fractional values as
	// specified by IEEE 754.
	Float float64

	// ID represents a unique identifier that is Base64 obfuscated. It
	// is often used to refetch an object or as key for a cache. The ID
	// type appears in a JSON response as a String; however, it is not
	// intended to be human-readable. When expected as an input type,
	// any string (such as "VXNlci0xMA==") or integer (such as 4) input
	// value will be accepted as an ID.
	ID any

	// Int represents non-fractional signed whole numeric values.
	// Int can represent values between -(2^31) and 2^31 - 1.
	Int int32

	// String represents textual data as UTF-8 character sequences.
	// This type is most often used by GraphQL to represent free-form
	// human-readable text.
	String string
)

// NewBoolean is a helper to make a new *Boolean.
func NewBoolean(v Boolean) *Boolean { return &v }

// NewFloat is a helper to make a new *Float.
func NewFloat(v Float) *Float { return &v }

// NewID is a helper to make a new *ID.
func NewID(v ID) *ID { return &v }

// NewInt is a helper to make a new *Int.
func NewInt(v Int) *Int { return &v }

// NewString is a helper to make a new *String.
func NewString(v String) *String { return &v }
