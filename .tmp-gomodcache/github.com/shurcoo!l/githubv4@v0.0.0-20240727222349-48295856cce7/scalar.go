package githubv4

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/shurcooL/graphql"
)

// Note: These custom types are meant to be used in queries for now.
// But the plan is to switch to using native Go types (string, int, bool, time.Time, etc.).
// See https://github.com/shurcooL/githubv4/issues/9 for details.
//
// These custom types currently provide documentation, and their use
// is required for sending outbound queries. However, native Go types
// can be used for unmarshaling. Once https://github.com/shurcooL/githubv4/issues/9
// is resolved, native Go types can completely replace these.

type (
	// Base64String is a (potentially binary) string encoded using base64.
	Base64String string

	// Boolean represents true or false values.
	Boolean graphql.Boolean

	// Date is an ISO-8601 encoded date.
	Date struct{ time.Time }

	// DateTime is an ISO-8601 encoded UTC date.
	DateTime struct{ time.Time }

	// Float represents signed double-precision fractional values as
	// specified by IEEE 754.
	Float graphql.Float

	// GitObjectID is a Git object ID. For example,
	// "912ec1990bd09f8fc128c3fa6b59105085aabc03".
	GitObjectID string

	// GitRefname is a fully qualified reference name (e.g., refs/heads/main).
	GitRefname string

	// GitTimestamp is an ISO-8601 encoded date.
	// Unlike the DateTime type, GitTimestamp is not converted in UTC.
	GitTimestamp struct{ time.Time }

	// HTML is a string containing HTML code.
	HTML string

	// ID represents a unique identifier that is Base64 obfuscated. It
	// is often used to refetch an object or as key for a cache. The ID
	// type appears in a JSON response as a String; however, it is not
	// intended to be human-readable. When expected as an input type,
	// any string (such as "VXNlci0xMA==") or integer (such as 4) input
	// value will be accepted as an ID.
	ID graphql.ID

	// Int represents non-fractional signed whole numeric values.
	// Int can represent values between -(2^31) and 2^31 - 1.
	Int graphql.Int

	// String represents textual data as UTF-8 character sequences.
	// This type is most often used by GraphQL to represent free-form
	// human-readable text.
	String graphql.String

	// URI is an RFC 3986, RFC 3987, and RFC 6570 (level 4) compliant URI.
	URI struct{ *url.URL }

	// X509Certificate is a valid x509 certificate.
	X509Certificate struct{ *x509.Certificate }
)

// MarshalJSON implements the json.Marshaler interface.
// The URI is a quoted string.
func (u URI) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The URI is expected to be a quoted string.
func (u *URI) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if string(data) == "null" {
		return nil
	}
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	u.URL, err = url.Parse(s)
	return err
}

// MarshalJSON implements the json.Marshaler interface.
func (x X509Certificate) MarshalJSON() ([]byte, error) {
	// TODO: Implement.
	return nil, fmt.Errorf("X509Certificate.MarshalJSON: not implemented")
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (x *X509Certificate) UnmarshalJSON(data []byte) error {
	// TODO: Implement.
	return fmt.Errorf("X509Certificate.UnmarshalJSON: not implemented")
}

// NewBase64String is a helper to make a new *Base64String.
func NewBase64String(v Base64String) *Base64String { return &v }

// NewBoolean is a helper to make a new *Boolean.
func NewBoolean(v Boolean) *Boolean { return &v }

// NewDate is a helper to make a new *Date.
func NewDate(v Date) *Date { return &v }

// NewDateTime is a helper to make a new *DateTime.
func NewDateTime(v DateTime) *DateTime { return &v }

// NewFloat is a helper to make a new *Float.
func NewFloat(v Float) *Float { return &v }

// NewGitObjectID is a helper to make a new *GitObjectID.
func NewGitObjectID(v GitObjectID) *GitObjectID { return &v }

// NewGitRefname is a helper to make a new *GitRefname.
func NewGitRefname(v GitRefname) *GitRefname { return &v }

// NewGitTimestamp is a helper to make a new *GitTimestamp.
func NewGitTimestamp(v GitTimestamp) *GitTimestamp { return &v }

// NewHTML is a helper to make a new *HTML.
func NewHTML(v HTML) *HTML { return &v }

// NewID is a helper to make a new *ID.
func NewID(v ID) *ID { return &v }

// NewInt is a helper to make a new *Int.
func NewInt(v Int) *Int { return &v }

// NewString is a helper to make a new *String.
func NewString(v String) *String { return &v }

// NewURI is a helper to make a new *URI.
func NewURI(v URI) *URI { return &v }

// NewX509Certificate is a helper to make a new *X509Certificate.
func NewX509Certificate(v X509Certificate) *X509Certificate { return &v }
