package githubv4_test

import (
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/shurcooL/githubv4"
)

func TestURI_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   githubv4.URI
		want string
	}{
		{
			in:   githubv4.URI{URL: &url.URL{Scheme: "https", Host: "example.org", Path: "/foo/bar"}},
			want: `"https://example.org/foo/bar"`,
		},
	}
	for _, tc := range tests {
		got, err := json.Marshal(tc.in)
		if err != nil {
			t.Fatalf("%s: got error: %v", tc.name, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got: %q, want: %q", tc.name, string(got), tc.want)
		}
	}
}

func TestURI_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		want      githubv4.URI
		wantError error
	}{
		{
			in:   `"https://example.org/foo/bar"`,
			want: githubv4.URI{URL: &url.URL{Scheme: "https", Host: "example.org", Path: "/foo/bar"}},
		},
		{
			name: "null",
			in:   `null`,
			want: githubv4.URI{},
		},
		{
			name:      "error JSON unmarshaling into string",
			in:        `86`,
			wantError: errors.New("json: cannot unmarshal number into Go value of type string"),
		},
	}
	for _, tc := range tests {
		var got githubv4.URI
		err := json.Unmarshal([]byte(tc.in), &got)
		if got, want := err, tc.wantError; !equalError(got, want) {
			t.Fatalf("%s: got error: %v, want: %v", tc.name, got, want)
		}
		if tc.wantError != nil {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got: %v, want: %v", tc.name, got, tc.want)
		}
	}
}

// equalError reports whether errors a and b are considered equal.
// They're equal if both are nil, or both are not nil and a.Error() == b.Error().
func equalError(a, b error) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Error() == b.Error()
}

func TestNewScalars(t *testing.T) {
	if got := githubv4.NewBase64String(""); got == nil {
		t.Error("NewBase64String returned nil")
	}
	if got := githubv4.NewBoolean(false); got == nil {
		t.Error("NewBoolean returned nil")
	}
	if got := githubv4.NewDate(githubv4.Date{}); got == nil {
		t.Error("NewDate returned nil")
	}
	if got := githubv4.NewDateTime(githubv4.DateTime{}); got == nil {
		t.Error("NewDateTime returned nil")
	}
	if got := githubv4.NewFloat(0.0); got == nil {
		t.Error("NewFloat returned nil")
	}
	if got := githubv4.NewGitObjectID(""); got == nil {
		t.Error("NewGitObjectID returned nil")
	}
	if got := githubv4.NewGitRefname(""); got == nil {
		t.Error("NewGitRefname returned nil")
	}
	if got := githubv4.NewGitTimestamp(githubv4.GitTimestamp{}); got == nil {
		t.Error("NewGitTimestamp returned nil")
	}
	if got := githubv4.NewHTML(""); got == nil {
		t.Error("NewHTML returned nil")
	}
	// ID with underlying type string.
	if got := githubv4.NewID(""); got == nil {
		t.Error("NewID returned nil")
	}
	// ID with underlying type int.
	if got := githubv4.NewID(0); got == nil {
		t.Error("NewID returned nil")
	}
	if got := githubv4.NewInt(0); got == nil {
		t.Error("NewInt returned nil")
	}
	if got := githubv4.NewString(""); got == nil {
		t.Error("NewString returned nil")
	}
	if got := githubv4.NewURI(githubv4.URI{}); got == nil {
		t.Error("NewURI returned nil")
	}
	if got := githubv4.NewX509Certificate(githubv4.X509Certificate{}); got == nil {
		t.Error("NewX509Certificate returned nil")
	}
}
