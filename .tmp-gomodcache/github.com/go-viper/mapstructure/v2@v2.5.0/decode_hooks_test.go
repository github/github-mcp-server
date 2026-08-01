package mapstructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type decodeHookTestSuite[F any, T any] struct {
	fn   DecodeHookFunc
	ok   []decodeHookTestCase[F, T]
	fail []decodeHookFailureTestCase[F, T]
}

func (ts decodeHookTestSuite[F, T]) Run(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		for _, tc := range ts.ok {
			tc := tc

			t.Run("", func(t *testing.T) {
				t.Parallel()

				tc.Run(t, ts.fn)
			})
		}
	})

	t.Run("Fail", func(t *testing.T) {
		t.Parallel()

		for _, tc := range ts.fail {
			tc := tc

			t.Run("", func(t *testing.T) {
				t.Parallel()

				tc.Run(t, ts.fn)
			})
		}
	})

	t.Run("NoOp", func(t *testing.T) {
		t.Parallel()

		var zero F

		actual, err := DecodeHookExec(ts.fn, reflect.ValueOf(zero), reflect.ValueOf(zero))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if !reflect.DeepEqual(actual, zero) {
			t.Fatalf("expected %[1]T(%#[1]v), got %[2]T(%#[2]v)", zero, actual)
		}
	})
}

type decodeHookTestCase[F any, T any] struct {
	from     F
	expected T
}

func (tc decodeHookTestCase[F, T]) Run(t *testing.T, fn DecodeHookFunc) {
	var to T

	actual, err := DecodeHookExec(fn, reflect.ValueOf(tc.from), reflect.ValueOf(to))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !reflect.DeepEqual(actual, tc.expected) {
		t.Fatalf("expected %[1]T(%#[1]v), got %[2]T(%#[2]v)", tc.expected, actual)
	}
}

type decodeHookFailureTestCase[F any, T any] struct {
	from F
}

func (tc decodeHookFailureTestCase[F, T]) Run(t *testing.T, fn DecodeHookFunc) {
	var to T

	_, err := DecodeHookExec(fn, reflect.ValueOf(tc.from), reflect.ValueOf(to))
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestComposeDecodeHookFunc(t *testing.T) {
	f1 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return data.(string) + "foo", nil
	}

	f2 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return data.(string) + "bar", nil
	}

	f := ComposeDecodeHookFunc(f1, f2)

	result, err := DecodeHookExec(
		f, reflect.ValueOf(""), reflect.ValueOf([]byte("")))
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if result.(string) != "foobar" {
		t.Fatalf("bad: %#v", result)
	}
}

func TestComposeDecodeHookFunc_err(t *testing.T) {
	f1 := func(reflect.Kind, reflect.Kind, any) (any, error) {
		return nil, errors.New("foo")
	}

	f2 := func(reflect.Kind, reflect.Kind, any) (any, error) {
		panic("NOPE")
	}

	f := ComposeDecodeHookFunc(f1, f2)

	_, err := DecodeHookExec(
		f, reflect.ValueOf(""), reflect.ValueOf([]byte("")))
	if err.Error() != "foo" {
		t.Fatalf("bad: %s", err)
	}
}

func TestComposeDecodeHookFunc_kinds(t *testing.T) {
	var f2From reflect.Kind

	f1 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return int(42), nil
	}

	f2 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		f2From = f
		return data, nil
	}

	f := ComposeDecodeHookFunc(f1, f2)

	_, err := DecodeHookExec(
		f, reflect.ValueOf(""), reflect.ValueOf([]byte("")))
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if f2From != reflect.Int {
		t.Fatalf("bad: %#v", f2From)
	}
}

func TestOrComposeDecodeHookFunc(t *testing.T) {
	f1 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return data.(string) + "foo", nil
	}

	f2 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return data.(string) + "bar", nil
	}

	f := OrComposeDecodeHookFunc(f1, f2)

	result, err := DecodeHookExec(
		f, reflect.ValueOf(""), reflect.ValueOf([]byte("")))
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if result.(string) != "foo" {
		t.Fatalf("bad: %#v", result)
	}
}

func TestOrComposeDecodeHookFunc_correctValueIsLast(t *testing.T) {
	f1 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return nil, errors.New("f1 error")
	}

	f2 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return nil, errors.New("f2 error")
	}

	f3 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return data.(string) + "bar", nil
	}

	f := OrComposeDecodeHookFunc(f1, f2, f3)

	result, err := DecodeHookExec(
		f, reflect.ValueOf(""), reflect.ValueOf([]byte("")))
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if result.(string) != "bar" {
		t.Fatalf("bad: %#v", result)
	}
}

func TestOrComposeDecodeHookFunc_err(t *testing.T) {
	f1 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return nil, errors.New("f1 error")
	}

	f2 := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return nil, errors.New("f2 error")
	}

	f := OrComposeDecodeHookFunc(f1, f2)

	_, err := DecodeHookExec(
		f, reflect.ValueOf(""), reflect.ValueOf([]byte("")))
	if err == nil {
		t.Fatalf("bad: should return an error")
	}
	if err.Error() != "f1 error\nf2 error\n" {
		t.Fatalf("bad: %s", err)
	}
}

func TestComposeDecodeHookFunc_safe_nofuncs(t *testing.T) {
	f := ComposeDecodeHookFunc()
	type myStruct2 struct {
		MyInt int
	}

	type myStruct1 struct {
		Blah map[string]myStruct2
	}

	src := &myStruct1{Blah: map[string]myStruct2{
		"test": {
			MyInt: 1,
		},
	}}

	dst := &myStruct1{}
	dConf := &DecoderConfig{
		Result:      dst,
		ErrorUnused: true,
		DecodeHook:  f,
	}
	d, err := NewDecoder(dConf)
	if err != nil {
		t.Fatal(err)
	}
	err = d.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
}

func TestComposeDecodeHookFunc_ReflectValueHook(t *testing.T) {
	reflectValueHook := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		new := data.(string) + "foo"
		return reflect.ValueOf(new), nil
	}

	stringHook := func(
		f reflect.Kind,
		t reflect.Kind,
		data any,
	) (any, error) {
		return data.(string) + "bar", nil
	}

	f := ComposeDecodeHookFunc(reflectValueHook, stringHook)

	result, err := DecodeHookExec(
		f, reflect.ValueOf(""), reflect.ValueOf([]byte("")))
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if result.(string) != "foobar" {
		t.Fatalf("bad: %#v", result)
	}
}

func TestStringToSliceHookFunc(t *testing.T) {
	// Test comma separator
	commaSuite := decodeHookTestSuite[string, []string]{
		fn: StringToSliceHookFunc(","),
		ok: []decodeHookTestCase[string, []string]{
			{"foo,bar,baz", []string{"foo", "bar", "baz"}}, // Basic comma separation
			{"", []string{}},                                                                    // Empty string
			{"single", []string{"single"}},                                                      // Single element
			{"one,two", []string{"one", "two"}},                                                 // Two elements
			{"foo, bar, baz", []string{"foo", " bar", " baz"}},                                  // Preserves spaces
			{"foo,,bar", []string{"foo", "", "bar"}},                                            // Empty elements
			{",foo,bar,", []string{"", "foo", "bar", ""}},                                       // Leading/trailing separators
			{"foo,bar,baz,", []string{"foo", "bar", "baz", ""}},                                 // Trailing separator
			{",foo", []string{"", "foo"}},                                                       // Leading separator only
			{"foo,", []string{"foo", ""}},                                                       // Trailing separator only
			{"a,b,c,d,e,f,g,h,i,j", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}}, // Many elements
		},
		fail: []decodeHookFailureTestCase[string, []string]{
			// StringToSliceHookFunc doesn't have failure cases - it always succeeds
		},
	}
	t.Run("CommaSeparator", commaSuite.Run)

	// Test semicolon separator
	semicolonSuite := decodeHookTestSuite[string, []string]{
		fn: StringToSliceHookFunc(";"),
		ok: []decodeHookTestCase[string, []string]{
			{"foo;bar;baz", []string{"foo", "bar", "baz"}}, // Basic semicolon separation
			{"", []string{}},                                   // Empty string
			{"single", []string{"single"}},                     // Single element
			{"one;two", []string{"one", "two"}},                // Two elements
			{"foo; bar; baz", []string{"foo", " bar", " baz"}}, // Preserves spaces
			{"foo;;bar", []string{"foo", "", "bar"}},           // Empty elements
			{";foo;bar;", []string{"", "foo", "bar", ""}},      // Leading/trailing separators
		},
		fail: []decodeHookFailureTestCase[string, []string]{},
	}
	t.Run("SemicolonSeparator", semicolonSuite.Run)

	// Test pipe separator
	pipeSuite := decodeHookTestSuite[string, []string]{
		fn: StringToSliceHookFunc("|"),
		ok: []decodeHookTestCase[string, []string]{
			{"foo|bar|baz", []string{"foo", "bar", "baz"}}, // Basic pipe separation
			{"", []string{}},                         // Empty string
			{"single", []string{"single"}},           // Single element
			{"foo||bar", []string{"foo", "", "bar"}}, // Empty elements
		},
		fail: []decodeHookFailureTestCase[string, []string]{},
	}
	t.Run("PipeSeparator", pipeSuite.Run)

	// Test space separator
	spaceSuite := decodeHookTestSuite[string, []string]{
		fn: StringToSliceHookFunc(" "),
		ok: []decodeHookTestCase[string, []string]{
			{"foo bar baz", []string{"foo", "bar", "baz"}}, // Basic space separation
			{"", []string{}},                         // Empty string
			{"single", []string{"single"}},           // Single element
			{"foo  bar", []string{"foo", "", "bar"}}, // Double space creates empty element
		},
		fail: []decodeHookFailureTestCase[string, []string]{},
	}
	t.Run("SpaceSeparator", spaceSuite.Run)

	// Test multi-character separator
	multiCharSuite := decodeHookTestSuite[string, []string]{
		fn: StringToSliceHookFunc("::"),
		ok: []decodeHookTestCase[string, []string]{
			{"foo::bar::baz", []string{"foo", "bar", "baz"}}, // Basic multi-char separation
			{"", []string{}},                                 // Empty string
			{"single", []string{"single"}},                   // Single element
			{"foo::::bar", []string{"foo", "", "bar"}},       // Double separator creates empty element
			{"::foo::bar::", []string{"", "foo", "bar", ""}}, // Leading/trailing separators
		},
		fail: []decodeHookFailureTestCase[string, []string]{},
	}
	t.Run("MultiCharSeparator", multiCharSuite.Run)

	// Test edge cases with custom logic for type conversion
	t.Run("NonStringTypes", func(t *testing.T) {
		f := StringToSliceHookFunc(",")

		// Test that non-string types are passed through unchanged
		sliceValue := reflect.ValueOf([]string{"42"})
		actual, err := DecodeHookExec(f, sliceValue, sliceValue)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(actual, []string{"42"}) {
			t.Fatalf("expected %v, got %v", []string{"42"}, actual)
		}

		// Test byte slice passthrough
		byteValue := reflect.ValueOf([]byte("42"))
		actual, err = DecodeHookExec(f, byteValue, reflect.ValueOf([]byte{}))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(actual, []byte("42")) {
			t.Fatalf("expected %v, got %v", []byte("42"), actual)
		}

		// Test string to string passthrough
		strValue := reflect.ValueOf("42")
		actual, err = DecodeHookExec(f, strValue, strValue)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(actual, "42") {
			t.Fatalf("expected %v, got %v", "42", actual)
		}
	})
}

func TestStringToWeakSliceHookFunc(t *testing.T) {
	f := StringToWeakSliceHookFunc(",")

	strValue := reflect.ValueOf("42")
	sliceValue := reflect.ValueOf([]string{"42"})
	sliceValue2 := reflect.ValueOf([]byte("42"))

	cases := []struct {
		f, t   reflect.Value
		result any
		err    bool
	}{
		{sliceValue, sliceValue, []string{"42"}, false},
		{sliceValue2, sliceValue2, []byte("42"), false},
		{reflect.ValueOf([]byte("42")), reflect.ValueOf([]byte{}), []byte("42"), false},
		{strValue, strValue, "42", false},
		{
			reflect.ValueOf("foo,bar,baz"),
			sliceValue,
			[]string{"foo", "bar", "baz"},
			false,
		},
		{
			reflect.ValueOf("foo,bar,baz"),
			sliceValue2,
			[]string{"foo", "bar", "baz"},
			false,
		},
		{
			reflect.ValueOf(""),
			sliceValue,
			[]string{},
			false,
		},
		{
			reflect.ValueOf(""),
			sliceValue2,
			[]string{},
			false,
		},
	}

	for i, tc := range cases {
		actual, err := DecodeHookExec(f, tc.f, tc.t)

		if tc.err != (err != nil) {
			t.Fatalf("case %d: expected err %#v", i, tc.err)
		}

		if !reflect.DeepEqual(actual, tc.result) {
			t.Fatalf(
				"case %d: expected %#v, got %#v",
				i, tc.result, actual)
		}
	}
}

func TestStringToTimeDurationHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, time.Duration]{
		fn: StringToTimeDurationHookFunc(),
		ok: []decodeHookTestCase[string, time.Duration]{
			// Basic units
			{"5s", 5 * time.Second},            // Seconds
			{"10ms", 10 * time.Millisecond},    // Milliseconds
			{"100us", 100 * time.Microsecond},  // Microseconds
			{"1000ns", 1000 * time.Nanosecond}, // Nanoseconds
			{"2m", 2 * time.Minute},            // Minutes
			{"3h", 3 * time.Hour},              // Hours
			{"24h", 24 * time.Hour},            // Day in hours

			// Combinations
			{"1h30m", time.Hour + 30*time.Minute},                       // Hour and minutes
			{"2h45m30s", 2*time.Hour + 45*time.Minute + 30*time.Second}, // Multiple units
			{"1m30s", time.Minute + 30*time.Second},                     // Minutes and seconds
			{"500ms", 500 * time.Millisecond},                           // Milliseconds only
			{"1.5s", time.Second + 500*time.Millisecond},                // Fractional seconds
			{"2.5h", 2*time.Hour + 30*time.Minute},                      // Fractional hours

			// Zero values
			{"0s", 0},  // Zero seconds
			{"0ms", 0}, // Zero milliseconds
			{"0h", 0},  // Zero hours
			{"0", 0},   // Just zero

			// Negative durations
			{"-5s", -5 * time.Second},                 // Negative seconds
			{"-1h30m", -(time.Hour + 30*time.Minute)}, // Negative combined
			{"-100ms", -100 * time.Millisecond},       // Negative milliseconds

			// Fractional values
			{"0.5s", 500 * time.Millisecond},                     // Half second
			{"1.25m", time.Minute + 15*time.Second},              // Fractional minute
			{"0.1h", 6 * time.Minute},                            // Fractional hour
			{"2.5ms", 2*time.Millisecond + 500*time.Microsecond}, // Fractional millisecond

			// Large values
			{"8760h", 8760 * time.Hour},               // 1 year in hours
			{"525600m", 525600 * time.Minute},         // 1 year in minutes
			{"1000000us", 1000000 * time.Microsecond}, // Large microseconds

			// Additional valid cases
			{".5s", 500 * time.Millisecond},            // Leading decimal is valid
			{"5µs", 5 * time.Microsecond},              // Unicode micro symbol is valid
			{"5.s", 5 * time.Second},                   // Trailing decimal is valid
			{"5s5m5s", 10*time.Second + 5*time.Minute}, // Duplicate units are valid
		},
		fail: []decodeHookFailureTestCase[string, time.Duration]{
			{"5"},        // Missing unit
			{"abc"},      // Invalid format
			{""},         // Empty string
			{"5x"},       // Invalid unit
			{"5ss"},      // Double unit letters
			{"5..5s"},    // Multiple decimal points
			{"++5s"},     // Double plus sign
			{"--5s"},     // Double minus sign
			{" 5s "},     // Leading/trailing whitespace not handled
			{"\t10ms\n"}, // Tab/newline whitespace not handled
			{"\r1h\r"},   // Carriage return whitespace not handled
			{"5s "},      // Trailing space after unit
			{" 5 s"},     // Space before unit
			{"5 s 10 m"}, // Spaces in combined duration
			{"∞s"},       // Unicode infinity symbol
			{"1y"},       // Unsupported unit (years)
			{"1w"},       // Unsupported unit (weeks)
			{"1d"},       // Unsupported unit (days)
		},
	}

	// Test non-string and non-duration type passthrough
	t.Run("Passthrough", func(t *testing.T) {
		f := StringToTimeDurationHookFunc()

		// Non-string type should pass through
		intValue := reflect.ValueOf(42)
		actual, err := DecodeHookExec(f, intValue, reflect.ValueOf(time.Duration(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if actual != 42 {
			t.Fatalf("expected 42, got %v", actual)
		}

		// Non-duration target type should pass through
		strValue := reflect.ValueOf("5s")
		actual, err = DecodeHookExec(f, strValue, strValue)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if actual != "5s" {
			t.Fatalf("expected '5s', got %v", actual)
		}
	})

	suite.Run(t)
}

func TestStringToTimeLocationHookFunc(t *testing.T) {
	newYork, _ := time.LoadLocation("America/New_York")
	london, _ := time.LoadLocation("Europe/London")
	tehran, _ := time.LoadLocation("Asia/Tehran")
	shanghai, _ := time.LoadLocation("Asia/Shanghai")

	suite := decodeHookTestSuite[string, *time.Location]{
		fn: StringToTimeLocationHookFunc(),
		ok: []decodeHookTestCase[string, *time.Location]{
			{"UTC", time.UTC},
			{"Local", time.Local},
			{"America/New_York", newYork},
			{"Europe/London", london},
			{"Asia/Tehran", tehran},
			{"Asia/Shanghai", shanghai},
		},
		fail: []decodeHookFailureTestCase[string, *time.Location]{
			{"UTC2"},           // Non-existent
			{"5s"},             // Duration-like, not a zone
			{"Europe\\London"}, // Invalid path separator
			{"../etc/passwd"},  // Unsafe path
			{"/etc/zoneinfo"},  // Absolute path (rejected by stdlib)
			{"Asia\\Tehran"},   // Invalid Windows-style path
		},
	}

	suite.Run(t)
}

func TestStringToURLHookFunc(t *testing.T) {
	httpURL, _ := url.Parse("http://example.com")
	httpsURL, _ := url.Parse("https://example.com")
	ftpURL, _ := url.Parse("ftp://ftp.example.com")
	fileURL, _ := url.Parse("file:///path/to/file")
	complexURL, _ := url.Parse("https://user:pass@example.com:8080/path?query=value&foo=bar#fragment")
	ipURL, _ := url.Parse("http://192.168.1.1:8080")
	ipv6URL, _ := url.Parse("http://[::1]:8080")
	emptyURL, _ := url.Parse("")

	suite := decodeHookTestSuite[string, *url.URL]{
		fn: StringToURLHookFunc(),
		ok: []decodeHookTestCase[string, *url.URL]{
			{"http://example.com", httpURL},   // Basic HTTP URL
			{"https://example.com", httpsURL}, // HTTPS URL
			{"ftp://ftp.example.com", ftpURL}, // FTP URL
			{"file:///path/to/file", fileURL}, // File URL
			{"https://user:pass@example.com:8080/path?query=value&foo=bar#fragment", complexURL}, // Complex URL with all components
			{"http://192.168.1.1:8080", ipURL},                                                   // IPv4 address with port
			{"http://[::1]:8080", ipv6URL},                                                       // IPv6 address with port
			{"", emptyURL},                                                                       // Empty URL
			// Additional valid cases that url.Parse accepts
			{"http://", func() *url.URL { u, _ := url.Parse("http://"); return u }()},                                   // Scheme with empty host
			{"http://example.com:99999", func() *url.URL { u, _ := url.Parse("http://example.com:99999"); return u }()}, // High port number
			{"not a url at all", func() *url.URL { u, _ := url.Parse("not a url at all"); return u }()},                 // Relative path (valid)
		},
		fail: []decodeHookFailureTestCase[string, *url.URL]{
			{"http ://example.com"},  // Space in scheme
			{"://invalid"},           // Missing scheme
			{"http://[invalid:ipv6"}, // Malformed IPv6 bracket
		},
	}

	suite.Run(t)
}

func TestStringToTimeHookFunc(t *testing.T) {
	strValue := reflect.ValueOf("5")
	timeValue := reflect.ValueOf(time.Time{})
	cases := []struct {
		f, t   reflect.Value
		layout string
		result any
		err    bool
	}{
		{
			reflect.ValueOf("2006-01-02T15:04:05Z"), timeValue, time.RFC3339,
			time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC), false,
		},
		{strValue, timeValue, time.RFC3339, time.Time{}, true},
		{strValue, strValue, time.RFC3339, "5", false},
	}

	for i, tc := range cases {
		f := StringToTimeHookFunc(tc.layout)
		actual, err := DecodeHookExec(f, tc.f, tc.t)
		if tc.err != (err != nil) {
			t.Fatalf("case %d: expected err %#v", i, tc.err)
		}
		if !reflect.DeepEqual(actual, tc.result) {
			t.Fatalf(
				"case %d: expected %#v, got %#v",
				i, tc.result, actual)
		}
	}
}

func TestStringToIPHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, net.IP]{
		fn: StringToIPHookFunc(),
		ok: []decodeHookTestCase[string, net.IP]{
			// IPv4 addresses
			{"1.2.3.4", net.IPv4(0x01, 0x02, 0x03, 0x04)},     // Basic IPv4
			{"192.168.1.1", net.IPv4(192, 168, 1, 1)},         // Private network address
			{"0.0.0.0", net.IPv4(0, 0, 0, 0)},                 // Zero address
			{"255.255.255.255", net.IPv4(255, 255, 255, 255)}, // Broadcast address
			{"127.0.0.1", net.IPv4(127, 0, 0, 1)},             // Localhost
			{"10.0.0.1", net.IPv4(10, 0, 0, 1)},               // Private network
			// IPv6 addresses
			{"::1", net.ParseIP("::1")},                 // IPv6 localhost
			{"2001:db8::1", net.ParseIP("2001:db8::1")}, // Documentation address
			{"fe80::1", net.ParseIP("fe80::1")},         // Link-local address
			{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")}, // Full IPv6 address
			{"2001:db8:85a3::8a2e:370:7334", net.ParseIP("2001:db8:85a3::8a2e:370:7334")},                       // Compressed IPv6
			{"::", net.ParseIP("::")},                             // IPv6 zero address
			{"::ffff:192.0.2.1", net.ParseIP("::ffff:192.0.2.1")}, // IPv4-mapped IPv6
		},
		fail: []decodeHookFailureTestCase[string, net.IP]{
			{"5"},                 // Single number
			{"256.1.1.1"},         // IPv4 octet too large
			{"1.2.3"},             // Too few IPv4 octets
			{"1.2.3.4.5"},         // Too many IPv4 octets
			{"not.an.ip.address"}, // Non-numeric text
			{""},                  // Empty string
			{"192.168.1.256"},     // Last octet too large
			{"192.168.-1.1"},      // Negative octet
			{"gggg::1"},           // Invalid hex in IPv6
			{"2001:db8::1::2"},    // Double :: in IPv6
			{"[::1]"},             // IPv6 with brackets (not raw IP)
			{"192.168.1.1:8080"},  // IPv4 with port
		},
	}

	suite.Run(t)
}

func TestStringToIPNetHookFunc(t *testing.T) {
	strValue := reflect.ValueOf("5")
	ipNetValue := reflect.ValueOf(net.IPNet{})
	var nilNet *net.IPNet = nil

	cases := []struct {
		f, t   reflect.Value
		result any
		err    bool
	}{
		{
			reflect.ValueOf("1.2.3.4/24"), ipNetValue,
			&net.IPNet{
				IP:   net.IP{0x01, 0x02, 0x03, 0x00},
				Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0x00),
			}, false,
		},
		{strValue, ipNetValue, nilNet, true},
		{strValue, strValue, "5", false},
	}

	for i, tc := range cases {
		f := StringToIPNetHookFunc()
		actual, err := DecodeHookExec(f, tc.f, tc.t)
		if tc.err != (err != nil) {
			t.Fatalf("case %d: expected err %#v", i, tc.err)
		}
		if !reflect.DeepEqual(actual, tc.result) {
			t.Fatalf(
				"case %d: expected %#v, got %#v",
				i, tc.result, actual)
		}
	}
}

func TestWeaklyTypedHook(t *testing.T) {
	var f DecodeHookFunc = WeaklyTypedHook

	strValue := reflect.ValueOf("")
	cases := []struct {
		f, t   reflect.Value
		result any
		err    bool
	}{
		// TO STRING
		{
			reflect.ValueOf(false),
			strValue,
			"0", // bool false converts to "0"
			false,
		},

		{
			reflect.ValueOf(true),
			strValue,
			"1", // bool true converts to "1"
			false,
		},

		{
			reflect.ValueOf(float32(7)),
			strValue,
			"7", // float32 converts to string
			false,
		},

		{
			reflect.ValueOf(int(7)),
			strValue,
			"7", // int converts to string
			false,
		},

		{
			reflect.ValueOf([]uint8("foo")),
			strValue,
			"foo", // byte slice converts to string
			false,
		},

		{
			reflect.ValueOf(uint(7)),
			strValue,
			"7", // uint converts to string
			false,
		},
	}

	for i, tc := range cases {
		actual, err := DecodeHookExec(f, tc.f, tc.t)
		if tc.err != (err != nil) {
			t.Fatalf("case %d: expected err %#v", i, tc.err)
		}
		if !reflect.DeepEqual(actual, tc.result) {
			t.Fatalf(
				"case %d: expected %#v, got %#v",
				i, tc.result, actual)
		}
	}
}

func TestStructToMapHookFuncTabled(t *testing.T) {
	var f DecodeHookFunc = RecursiveStructToMapHookFunc()

	type b struct {
		TestKey string
	}

	type a struct {
		Sub b
	}

	testStruct := a{
		Sub: b{
			TestKey: "testval",
		},
	}

	testMap := map[string]any{
		"Sub": map[string]any{
			"TestKey": "testval",
		},
	}

	cases := []struct {
		name     string
		receiver any
		input    any
		expected any
		err      bool
	}{
		{
			"map receiver",
			func() any {
				var res map[string]any
				return &res
			}(),
			testStruct,
			&testMap,
			false,
		},
		{
			"interface receiver",
			func() any {
				var res any
				return &res
			}(),
			testStruct,
			func() any {
				var exp any = testMap
				return &exp
			}(),
			false,
		},
		{
			"slice receiver errors",
			func() any {
				var res []string
				return &res
			}(),
			testStruct,
			new([]string),
			true,
		},
		{
			"slice to slice - no change",
			func() any {
				var res []string
				return &res
			}(),
			[]string{"a", "b"},
			&[]string{"a", "b"},
			false,
		},
		{
			"string to string - no change",
			func() any {
				var res string
				return &res
			}(),
			"test",
			func() *string {
				s := "test"
				return &s
			}(),
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &DecoderConfig{
				DecodeHook: f,
				Result:     tc.receiver,
			}

			d, err := NewDecoder(cfg)
			if err != nil {
				t.Fatalf("unexpected err %#v", err)
			}

			err = d.Decode(tc.input)
			if tc.err != (err != nil) {
				t.Fatalf("expected err %#v", err)
			}

			if !reflect.DeepEqual(tc.expected, tc.receiver) {
				t.Fatalf("expected %#v, got %#v",
					tc.expected, tc.receiver)
			}
		})
	}
}

func TestTextUnmarshallerHookFunc(t *testing.T) {
	type MyString string

	cases := []struct {
		f, t   reflect.Value
		result any
		err    bool
	}{
		{reflect.ValueOf("42"), reflect.ValueOf(big.Int{}), big.NewInt(42), false},
		{reflect.ValueOf("invalid"), reflect.ValueOf(big.Int{}), nil, true},
		{reflect.ValueOf("5"), reflect.ValueOf("5"), "5", false},
		{reflect.ValueOf(json.Number("42")), reflect.ValueOf(big.Int{}), big.NewInt(42), false},
		{reflect.ValueOf(MyString("42")), reflect.ValueOf(big.Int{}), big.NewInt(42), false},
	}
	for i, tc := range cases {
		f := TextUnmarshallerHookFunc()
		actual, err := DecodeHookExec(f, tc.f, tc.t)
		if tc.err != (err != nil) {
			t.Fatalf("case %d: expected err %#v", i, tc.err)
		}
		if !reflect.DeepEqual(actual, tc.result) {
			t.Fatalf(
				"case %d: expected %#v, got %#v",
				i, tc.result, actual)
		}
	}
}

func TestStringToNetIPAddrHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, netip.Addr]{
		fn: StringToNetIPAddrHookFunc(),
		ok: []decodeHookTestCase[string, netip.Addr]{
			// IPv4 addresses
			{"192.0.2.1", netip.AddrFrom4([4]byte{0xc0, 0x00, 0x02, 0x01})},   // Documentation address
			{"127.0.0.1", netip.AddrFrom4([4]byte{127, 0, 0, 1})},             // Localhost
			{"0.0.0.0", netip.AddrFrom4([4]byte{0, 0, 0, 0})},                 // Zero address
			{"255.255.255.255", netip.AddrFrom4([4]byte{255, 255, 255, 255})}, // Broadcast address
			{"10.0.0.1", netip.AddrFrom4([4]byte{10, 0, 0, 1})},               // Private network
			{"192.168.1.100", netip.AddrFrom4([4]byte{192, 168, 1, 100})},     // Private network
			// IPv6 addresses
			{"::1", netip.AddrFrom16([16]byte{15: 1})},                                               // IPv6 localhost
			{"2001:db8::1", netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 12: 0, 0, 0, 1})},      // Documentation address
			{"fe80::1", netip.AddrFrom16([16]byte{0xfe, 0x80, 14: 0, 1})},                            // Link-local address
			{"::", netip.AddrFrom16([16]byte{})},                                                     // IPv6 zero address
			{"::ffff:192.0.2.1", netip.AddrFrom16([16]byte{10: 0xff, 0xff, 0xc0, 0x00, 0x02, 0x01})}, // IPv4-mapped IPv6
		},
		fail: []decodeHookFailureTestCase[string, netip.Addr]{
			{"5"},                 // Single number
			{"256.1.1.1"},         // IPv4 octet too large
			{"1.2.3"},             // Too few IPv4 octets
			{"1.2.3.4.5"},         // Too many IPv4 octets
			{"not.an.ip.address"}, // Non-numeric text
			{""},                  // Empty string
			{"192.168.1.256"},     // Last octet too large
			{"192.168.-1.1"},      // Negative octet
			{"gggg::1"},           // Invalid hex in IPv6
			{"2001:db8::1::2"},    // Double :: in IPv6
			{"[::1]"},             // IPv6 with brackets
			{"192.168.1.1:8080"},  // IPv4 with port
			{"192.168.1.1/24"},    // IPv4 with CIDR
		},
	}

	suite.Run(t)
}

func TestStringToNetIPAddrPortHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, netip.AddrPort]{
		fn: StringToNetIPAddrPortHookFunc(),
		ok: []decodeHookTestCase[string, netip.AddrPort]{
			// IPv4 with ports
			{"192.0.2.1:80", netip.AddrPortFrom(netip.AddrFrom4([4]byte{0xc0, 0x00, 0x02, 0x01}), 80)},         // HTTP port
			{"127.0.0.1:8080", netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 8080)},               // Alternative HTTP port
			{"10.0.0.1:443", netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, 0, 1}), 443)},                   // HTTPS port
			{"192.168.1.100:22", netip.AddrPortFrom(netip.AddrFrom4([4]byte{192, 168, 1, 100}), 22)},           // SSH port
			{"0.0.0.0:0", netip.AddrPortFrom(netip.AddrFrom4([4]byte{0, 0, 0, 0}), 0)},                         // Zero address and port
			{"255.255.255.255:65535", netip.AddrPortFrom(netip.AddrFrom4([4]byte{255, 255, 255, 255}), 65535)}, // Max address and port
			// IPv6 with ports
			{"[::1]:80", netip.AddrPortFrom(netip.AddrFrom16([16]byte{15: 1}), 80)},                                               // IPv6 localhost with HTTP
			{"[2001:db8::1]:443", netip.AddrPortFrom(netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 12: 0, 0, 0, 1}), 443)},    // Documentation address with HTTPS
			{"[fe80::1]:8080", netip.AddrPortFrom(netip.AddrFrom16([16]byte{0xfe, 0x80, 14: 0, 1}), 8080)},                        // Link-local with alt HTTP
			{"[::]:22", netip.AddrPortFrom(netip.AddrFrom16([16]byte{}), 22)},                                                     // IPv6 zero address with SSH
			{"[::ffff:192.0.2.1]:80", netip.AddrPortFrom(netip.AddrFrom16([16]byte{10: 0xff, 0xff, 0xc0, 0x00, 0x02, 0x01}), 80)}, // IPv4-mapped IPv6 with HTTP
		},
		fail: []decodeHookFailureTestCase[string, netip.AddrPort]{
			{"5"},                  // Just a number
			{"192.168.1.1"},        // Missing port
			{"192.168.1.1:"},       // Empty port
			{"192.168.1.1:65536"},  // Port too large
			{"192.168.1.1:-1"},     // Negative port
			{"192.168.1.1:abc"},    // Non-numeric port
			{"256.1.1.1:80"},       // Invalid IP
			{"::1:80"},             // IPv6 without brackets
			{"[::1"},               // Missing closing bracket
			{"::1]:80"},            // Missing opening bracket
			{"[invalid::ip]:80"},   // Invalid IPv6
			{""},                   // Empty string
			{":80"},                // Missing IP
			{"192.168.1.1:80:443"}, // Multiple ports
		},
	}

	suite.Run(t)
}

func TestStringToNetIPPrefixHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, netip.Prefix]{
		fn: StringToNetIPPrefixHookFunc(),
		ok: []decodeHookTestCase[string, netip.Prefix]{
			// IPv4 CIDR notation
			{"192.0.2.1/24", netip.PrefixFrom(netip.AddrFrom4([4]byte{0xc0, 0x00, 0x02, 0x01}), 24)},   // Documentation network
			{"127.0.0.1/32", netip.PrefixFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 32)},             // Localhost single host
			{"10.0.0.0/8", netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, 0, 0}), 8)},                 // Class A private network
			{"192.168.1.0/24", netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24)},         // Class C private network
			{"172.16.0.0/12", netip.PrefixFrom(netip.AddrFrom4([4]byte{172, 16, 0, 0}), 12)},           // Class B private network
			{"0.0.0.0/0", netip.PrefixFrom(netip.AddrFrom4([4]byte{0, 0, 0, 0}), 0)},                   // Default route
			{"255.255.255.255/32", netip.PrefixFrom(netip.AddrFrom4([4]byte{255, 255, 255, 255}), 32)}, // Broadcast single host
			{"192.168.1.1/30", netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 1}), 30)},         // Point-to-point subnet
			// IPv6 CIDR notation
			{"fd7a:115c::626b:430b/118", netip.PrefixFrom(netip.AddrFrom16([16]byte{0xfd, 0x7a, 0x11, 0x5c, 12: 0x62, 0x6b, 0x43, 0x0b}), 118)}, // ULA with specific prefix
			{"2001:db8::/32", netip.PrefixFrom(netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8}), 32)},                                         // Documentation network
			{"::1/128", netip.PrefixFrom(netip.AddrFrom16([16]byte{15: 1}), 128)},                                                               // IPv6 localhost single host
			{"::/0", netip.PrefixFrom(netip.AddrFrom16([16]byte{}), 0)},                                                                         // IPv6 default route
			{"fe80::/10", netip.PrefixFrom(netip.AddrFrom16([16]byte{0xfe, 0x80}), 10)},                                                         // Link-local network
			{"2001:db8::1/64", netip.PrefixFrom(netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 12: 0, 0, 0, 1}), 64)},                        // Standard IPv6 subnet
			{"::ffff:0:0/96", netip.PrefixFrom(netip.AddrFrom16([16]byte{10: 0xff, 0xff}), 96)},                                                 // IPv4-mapped IPv6 prefix
		},
		fail: []decodeHookFailureTestCase[string, netip.Prefix]{
			{"5"},                // Just a number
			{"192.168.1.1"},      // Missing prefix length
			{"192.168.1.1/"},     // Empty prefix length
			{"192.168.1.1/33"},   // IPv4 prefix too large
			{"192.168.1.1/-1"},   // Negative prefix
			{"192.168.1.1/abc"},  // Non-numeric prefix
			{"256.1.1.1/24"},     // Invalid IP
			{"::1/129"},          // IPv6 prefix too large
			{"invalid::ip/64"},   // Invalid IPv6
			{""},                 // Empty string
			{"/24"},              // Missing IP
			{"192.168.1.1/24/8"}, // Multiple prefixes
		},
	}

	suite.Run(t)
}

func TestStringToBasicTypeHookFunc(t *testing.T) {
	strValue := reflect.ValueOf("42")

	cases := []struct {
		f, t   reflect.Value
		result any
		err    bool
	}{
		{strValue, strValue, "42", false},
		{strValue, reflect.ValueOf(int8(0)), int8(42), false},
		{strValue, reflect.ValueOf(uint8(0)), uint8(42), false},
		{strValue, reflect.ValueOf(int16(0)), int16(42), false},
		{strValue, reflect.ValueOf(uint16(0)), uint16(42), false},
		{strValue, reflect.ValueOf(int32(0)), int32(42), false},
		{strValue, reflect.ValueOf(uint32(0)), uint32(42), false},
		{strValue, reflect.ValueOf(int64(0)), int64(42), false},
		{strValue, reflect.ValueOf(uint64(0)), uint64(42), false},
		{strValue, reflect.ValueOf(int(0)), int(42), false},
		{strValue, reflect.ValueOf(uint(0)), uint(42), false},
		{strValue, reflect.ValueOf(float32(0)), float32(42), false},
		{strValue, reflect.ValueOf(float64(0)), float64(42), false},
		{reflect.ValueOf("true"), reflect.ValueOf(bool(false)), true, false},
		{strValue, reflect.ValueOf(byte(0)), byte(42), false},
		{strValue, reflect.ValueOf(rune(0)), rune(42), false},
		{strValue, reflect.ValueOf(complex64(0)), complex64(42), false},
		{strValue, reflect.ValueOf(complex128(0)), complex128(42), false},
	}

	for i, tc := range cases {
		f := StringToBasicTypeHookFunc()
		actual, err := DecodeHookExec(f, tc.f, tc.t)
		if tc.err != (err != nil) {
			t.Fatalf("case %d: expected err %#v", i, tc.err)
		}
		if !tc.err && !reflect.DeepEqual(actual, tc.result) {
			t.Fatalf(
				"case %d: expected %#v, got %#v",
				i, tc.result, actual)
		}
	}
}

func TestStringToInt8HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, int8]{
		fn: StringToInt8HookFunc(),
		ok: []decodeHookTestCase[string, int8]{
			{"42", 42},             // Basic positive decimal
			{"-42", int8(-42)},     // Basic negative decimal
			{"0b101010", int8(42)}, // Binary notation
			{"052", int8(42)},      // Octal notation (legacy)
			{"0o52", int8(42)},     // Octal notation (modern)
			{"0x2a", int8(42)},     // Hex notation (lowercase)
			{"0X2A", int8(42)},     // Hex notation (uppercase)
			{"0", int8(0)},         // Zero
			{"+42", int8(42)},      // Explicit positive sign
			// Boundary values
			{"127", int8(127)},          // Max value
			{"-128", int8(-128)},        // Min value
			{"0x7F", int8(127)},         // Max value in hex
			{"-0x80", int8(-128)},       // Min value in hex
			{"0177", int8(127)},         // Max value in octal
			{"-0200", int8(-128)},       // Min value in octal
			{"0b01111111", int8(127)},   // Max value in binary
			{"-0b10000000", int8(-128)}, // Min value in binary
			// Zero variants
			{"+0", int8(0)},  // Explicit positive zero
			{"-0", int8(0)},  // Explicit negative zero
			{"00", int8(0)},  // Leading zero
			{"0x0", int8(0)}, // Hex zero
			{"0b0", int8(0)}, // Binary zero
			{"0o0", int8(0)}, // Octal zero
		},
		fail: []decodeHookFailureTestCase[string, int8]{
			{strings.Repeat("42", 42)}, // Very long number string
			{"128"},                    // Overflow
			{"-129"},                   // Underflow
			{"256"},                    // Way over max
			{"-256"},                   // Way under min
			{"42.5"},                   // Float
			{"abc"},                    // Non-numeric
			{""},                       // Empty string
			{" 42 "},                   // Whitespace not handled by strconv
			{"\t42\n"},                 // Whitespace not handled by strconv
			{"\r42\r"},                 // Whitespace not handled by strconv
			{"0x"},                     // Invalid hex
			{"0b"},                     // Invalid binary
			{"0o"},                     // Invalid octal
			{"++42"},                   // Double plus
			{"--42"},                   // Double minus
			{"42abc"},                  // Trailing non-numeric
			{"abc42"},                  // Leading non-numeric
			{"42 43"},                  // Multiple numbers
			{"0x10000"},                // Hex overflow
			{"-0x10001"},               // Hex underflow
			{"0777"},                   // Octal overflow
			{"-01000"},                 // Octal underflow
			{"0b100000000"},            // Binary overflow
			{"-0b100000001"},           // Binary underflow
		},
	}

	suite.Run(t)
}

func TestStringToUint8HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, uint8]{
		fn: StringToUint8HookFunc(),
		ok: []decodeHookTestCase[string, uint8]{
			{"42", 42},              // Basic decimal
			{"0b101010", uint8(42)}, // Binary notation
			{"052", uint8(42)},      // Octal notation (legacy)
			{"0o52", uint8(42)},     // Octal notation (modern)
			{"0x2a", uint8(42)},     // Hex notation (lowercase)
			{"0X2A", uint8(42)},     // Hex notation (uppercase)
			{"0", uint8(0)},         // Zero

			// Boundary values
			{"255", uint8(255)},        // Max value
			{"0xFF", uint8(255)},       // Max value in hex
			{"0377", uint8(255)},       // Max value in octal
			{"0b11111111", uint8(255)}, // Max value in binary
			{"1", uint8(1)},            // Min positive value
			// Zero variants

			{"00", uint8(0)},  // Leading zero
			{"0x0", uint8(0)}, // Hex zero
			{"0b0", uint8(0)}, // Binary zero
			{"0o0", uint8(0)}, // Octal zero
		},
		fail: []decodeHookFailureTestCase[string, uint8]{
			{strings.Repeat("42", 42)},
			{"256"},         // Overflow
			{"512"},         // Way over max
			{"42.5"},        // Float
			{"abc"},         // Non-numeric
			{""},            // Empty string
			{"-1"},          // Negative number
			{"-42"},         // Negative number
			{"0x"},          // Invalid hex
			{"0b"},          // Invalid binary
			{"0o"},          // Invalid octal
			{"++42"},        // Double plus
			{" 42 "},        // Whitespace not handled by strconv
			{"\t42\n"},      // Whitespace not handled by strconv
			{"\r42\r"},      // Whitespace not handled by strconv
			{"42abc"},       // Trailing non-numeric
			{"abc42"},       // Leading non-numeric
			{"42 43"},       // Multiple numbers
			{"0x100"},       // Hex overflow
			{"0400"},        // Octal overflow
			{"0b100000000"}, // Binary overflow
		},
	}

	suite.Run(t)
}

func TestStringToInt16HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, int16]{
		fn: StringToInt16HookFunc(),
		ok: []decodeHookTestCase[string, int16]{
			{"42", 42},
			{"-42", int16(-42)},
			{"0b101010", int16(42)},
			{"052", int16(42)},
			{"0o52", int16(42)},
			{"0x2a", int16(42)},
			{"0X2A", int16(42)},
			{"0", int16(0)},
		},
		fail: []decodeHookFailureTestCase[string, int16]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToUint16HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, uint16]{
		fn: StringToUint16HookFunc(),
		ok: []decodeHookTestCase[string, uint16]{
			{"42", 42},
			{"0b101010", uint16(42)},
			{"052", uint16(42)},
			{"0o52", uint16(42)},
			{"0x2a", uint16(42)},
			{"0X2A", uint16(42)},
			{"0", uint16(0)},
		},
		fail: []decodeHookFailureTestCase[string, uint16]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"-42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToInt32HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, int32]{
		fn: StringToInt32HookFunc(),
		ok: []decodeHookTestCase[string, int32]{
			{"42", 42},
			{"-42", int32(-42)},
			{"0b101010", int32(42)},
			{"052", int32(42)},
			{"0o52", int32(42)},
			{"0x2a", int32(42)},
			{"0X2A", int32(42)},
			{"0", int32(0)},
		},
		fail: []decodeHookFailureTestCase[string, int32]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToUint32HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, uint32]{
		fn: StringToUint32HookFunc(),
		ok: []decodeHookTestCase[string, uint32]{
			{"42", 42},
			{"0b101010", uint32(42)},
			{"052", uint32(42)},
			{"0o52", uint32(42)},
			{"0x2a", uint32(42)},
			{"0X2A", uint32(42)},
			{"0", uint32(0)},
		},
		fail: []decodeHookFailureTestCase[string, uint32]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"-42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToInt64HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, int64]{
		fn: StringToInt64HookFunc(),
		ok: []decodeHookTestCase[string, int64]{
			{"42", 42},
			{"-42", int64(-42)},
			{"0b101010", int64(42)},
			{"052", int64(42)},
			{"0o52", int64(42)},
			{"0x2a", int64(42)},
			{"0X2A", int64(42)},
			{"0", int64(0)},
		},
		fail: []decodeHookFailureTestCase[string, int64]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToUint64HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, uint64]{
		fn: StringToUint64HookFunc(),
		ok: []decodeHookTestCase[string, uint64]{
			{"42", 42},
			{"0b101010", uint64(42)},
			{"052", uint64(42)},
			{"0o52", uint64(42)},
			{"0x2a", uint64(42)},
			{"0X2A", uint64(42)},
			{"0", uint64(0)},
		},
		fail: []decodeHookFailureTestCase[string, uint64]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"-42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToIntHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, int]{
		fn: StringToIntHookFunc(),
		ok: []decodeHookTestCase[string, int]{
			{"42", 42},
			{"-42", int(-42)},
			{"0b101010", int(42)},
			{"052", int(42)},
			{"0o52", int(42)},
			{"0x2a", int(42)},
			{"0X2A", int(42)},
			{"0", int(0)},
		},
		fail: []decodeHookFailureTestCase[string, int]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToUintHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, uint]{
		fn: StringToUintHookFunc(),
		ok: []decodeHookTestCase[string, uint]{
			{"42", 42},
			{"0b101010", uint(42)},
			{"052", uint(42)},
			{"0o52", uint(42)},
			{"0x2a", uint(42)},
			{"0X2A", uint(42)},
			{"0", uint(0)},
		},
		fail: []decodeHookFailureTestCase[string, uint]{
			{strings.Repeat("42", 42)},
			{"42.42"},
			{"-42"},
			{"0.0"},
		},
	}

	suite.Run(t)
}

func TestStringToFloat32HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, float32]{
		fn: StringToFloat32HookFunc(),
		ok: []decodeHookTestCase[string, float32]{
			{"42.42", float32(42.42)},   // Basic decimal
			{"-42.42", float32(-42.42)}, // Negative decimal
			{"0", float32(0)},           // Zero as integer
			{"1e3", float32(1000)},      // Scientific notation
			{"1e-3", float32(0.001)},    // Small scientific notation
			// Integer values
			{"42", float32(42)},   // Positive integer
			{"-42", float32(-42)}, // Negative integer
			{"+42", float32(42)},  // Explicit positive integer
			// Zero variants
			{"0.0", float32(0.0)}, // Zero with decimal
			{"+0", float32(0)},    // Explicit positive zero
			{"-0", float32(0)},    // Explicit negative zero
			{"00.00", float32(0)}, // Zero with leading zeros
			// Scientific notation
			{"1E3", float32(1000)},        // Scientific notation (uppercase E)
			{"1.5e2", float32(150)},       // Fractional base with exponent
			{"1.5E2", float32(150)},       // Fractional base with uppercase E
			{"-1.5e2", float32(-150)},     // Negative fractional with exponent
			{"1e+3", float32(1000)},       // Explicit positive exponent
			{"1e-10", float32(1e-10)},     // Very small exponent
			{"3.14159", float32(3.14159)}, // Pi approximation
			// Special values - infinity
			{"inf", float32(math.Inf(1))},        // Infinity (lowercase)
			{"+inf", float32(math.Inf(1))},       // Positive infinity
			{"-inf", float32(math.Inf(-1))},      // Negative infinity
			{"Inf", float32(math.Inf(1))},        // Infinity (capitalized)
			{"+Inf", float32(math.Inf(1))},       // Positive infinity (capitalized)
			{"-Inf", float32(math.Inf(-1))},      // Negative infinity (capitalized)
			{"infinity", float32(math.Inf(1))},   // Infinity (full word)
			{"+infinity", float32(math.Inf(1))},  // Positive infinity (full word)
			{"-infinity", float32(math.Inf(-1))}, // Negative infinity (full word)
			{"Infinity", float32(math.Inf(1))},   // Infinity (full word capitalized)
			{"+Infinity", float32(math.Inf(1))},  // Positive infinity (full word capitalized)
			{"-Infinity", float32(math.Inf(-1))}, // Negative infinity (full word capitalized)
			// Decimal variations
			{".5", float32(0.5)},   // Leading decimal point
			{"-.5", float32(-0.5)}, // Negative leading decimal
			{"+.5", float32(0.5)},  // Positive leading decimal
			{"5.", float32(5.0)},   // Trailing decimal point
			{"-5.", float32(-5.0)}, // Negative trailing decimal
			{"+5.", float32(5.0)},  // Positive trailing decimal
			// Very small and large numbers
			{"1.1754943508222875e-38", float32(1.1754943508222875e-38)}, // Near min positive
			{"3.4028234663852886e+38", float32(3.4028234663852886e+38)}, // Near max

		},
		fail: []decodeHookFailureTestCase[string, float32]{
			{strings.Repeat("42", 420)},
			{"42.42.42"},
			{"abc"},      // Non-numeric
			{""},         // Empty string
			{"42abc"},    // Trailing non-numeric
			{"abc42"},    // Leading non-numeric
			{"42 43"},    // Multiple numbers
			{"++42"},     // Double plus
			{"--42"},     // Double minus
			{"1e"},       // Incomplete scientific notation
			{"1e+"},      // Incomplete scientific notation
			{"1e-"},      // Incomplete scientific notation
			{"1.2.3"},    // Multiple dots
			{"1..2"},     // Double dots
			{"."},        // Just a dot
			{" 42.5 "},   // Whitespace not handled by strconv
			{"\t42.5\n"}, // Whitespace not handled by strconv
			{"\r42.5\r"}, // Whitespace not handled by strconv
			{" 42.5 "},   // Whitespace not handled by strconv
			{"\t42.5\n"}, // Whitespace not handled by strconv
			{"\r42.5\r"}, // Whitespace not handled by strconv
			{"1e1e1"},    // Multiple exponents
			{"∞"},        // Unicode infinity
			{"NaΝ"},      // Unicode NaN lookalike
		},
	}

	// Custom test for NaN since NaN != NaN
	t.Run("NaN", func(t *testing.T) {
		f := StringToFloat32HookFunc()
		actual, err := DecodeHookExec(f, reflect.ValueOf("nan"), reflect.ValueOf(float32(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !math.IsNaN(float64(actual.(float32))) {
			t.Fatalf("expected NaN, got %v", actual)
		}
	})

	suite.Run(t)
}

func TestStringToFloat64HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, float64]{
		fn: StringToFloat64HookFunc(),
		ok: []decodeHookTestCase[string, float64]{
			{"42.42", float64(42.42)},   // Basic decimal
			{"-42.42", float64(-42.42)}, // Negative decimal
			{"0", float64(0)},           // Zero as integer
			{"0.0", float64(0)},         // Zero with decimal
			{"1e3", float64(1000)},      // Scientific notation
			{"1e-3", float64(0.001)},    // Small scientific notation
			// Integer values
			{"42", float64(42)},   // Positive integer
			{"-42", float64(-42)}, // Negative integer
			{"+42", float64(42)},  // Explicit positive integer
			// Zero variants
			{"+0", float64(0)},    // Explicit positive zero
			{"-0", float64(0)},    // Explicit negative zero
			{"00.00", float64(0)}, // Zero with leading zeros
			// Scientific notation
			{"1E3", float64(1000)},                            // Scientific notation (uppercase E)
			{"1.5e2", float64(150)},                           // Fractional base with exponent
			{"1.5E2", float64(150)},                           // Fractional base with uppercase E
			{"-1.5e2", float64(-150)},                         // Negative fractional with exponent
			{"1e+3", float64(1000)},                           // Explicit positive exponent
			{"1e-15", float64(1e-15)},                         // Very small exponent
			{"3.141592653589793", float64(3.141592653589793)}, // Pi with high precision
			// Special values - infinity
			{"inf", math.Inf(1)},        // Infinity (lowercase)
			{"+inf", math.Inf(1)},       // Positive infinity
			{"-inf", math.Inf(-1)},      // Negative infinity
			{"Inf", math.Inf(1)},        // Infinity (capitalized)
			{"+Inf", math.Inf(1)},       // Positive infinity (capitalized)
			{"-Inf", math.Inf(-1)},      // Negative infinity (capitalized)
			{"infinity", math.Inf(1)},   // Infinity (full word)
			{"+infinity", math.Inf(1)},  // Positive infinity (full word)
			{"-infinity", math.Inf(-1)}, // Negative infinity (full word)
			{"Infinity", math.Inf(1)},   // Infinity (full word capitalized)
			{"+Infinity", math.Inf(1)},  // Positive infinity (full word capitalized)
			{"-Infinity", math.Inf(-1)}, // Negative infinity (full word capitalized)
			// Decimal variations
			{".5", float64(0.5)},   // Leading decimal point
			{"-.5", float64(-0.5)}, // Negative leading decimal
			{"+.5", float64(0.5)},  // Positive leading decimal
			{"5.", float64(5.0)},   // Trailing decimal point
			{"-5.", float64(-5.0)}, // Negative trailing decimal
			{"+5.", float64(5.0)},  // Positive trailing decimal
			// Very small and large numbers
			{"2.2250738585072014e-308", float64(2.2250738585072014e-308)}, // Near min positive
			{"1.7976931348623157e+308", float64(1.7976931348623157e+308)}, // Near max
			{"4.9406564584124654e-324", float64(4.9406564584124654e-324)}, // Min positive subnormal

		},
		fail: []decodeHookFailureTestCase[string, float64]{
			{strings.Repeat("42", 420)},
			{"42.42.42"},
			{"abc"},   // Non-numeric
			{""},      // Empty string
			{"42abc"}, // Trailing non-numeric
			{"abc42"}, // Leading non-numeric
			{"42 43"}, // Multiple numbers
			{"++42"},  // Double plus
			{"--42"},  // Double minus
			{"1e"},    // Incomplete scientific notation
			{"1e+"},   // Incomplete scientific notation
			{"1e-"},   // Incomplete scientific notation
			{"1.2.3"}, // Multiple dots
			{"1..2"},  // Double dots
			{"."},     // Just a dot
			{"1e1e1"}, // Multiple exponents
			{"∞"},     // Unicode infinity
			{"NaΝ"},   // Unicode NaN lookalike
		},
	}

	// Custom test for NaN since NaN != NaN
	t.Run("NaN", func(t *testing.T) {
		f := StringToFloat64HookFunc()
		actual, err := DecodeHookExec(f, reflect.ValueOf("nan"), reflect.ValueOf(float64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !math.IsNaN(actual.(float64)) {
			t.Fatalf("expected NaN, got %v", actual)
		}
	})

	suite.Run(t)
}

func TestStringToComplex64HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, complex64]{
		fn: StringToComplex64HookFunc(),
		ok: []decodeHookTestCase[string, complex64]{
			// Standard complex numbers
			{"42.42+42.42i", complex(float32(42.42), float32(42.42))}, // Basic complex number
			{"1+2i", complex(float32(1), float32(2))},                 // Simple complex number
			{"-1-2i", complex(float32(-1), float32(-2))},              // Negative real and imaginary
			{"1-2i", complex(float32(1), float32(-2))},                // Positive real, negative imaginary
			{"-1+2i", complex(float32(-1), float32(2))},               // Negative real, positive imaginary
			// Real numbers only
			{"-42.42", complex(float32(-42.42), 0)}, // Negative real number
			{"42", complex(float32(42), 0)},         // Positive integer
			{"+42", complex(float32(42), 0)},        // Explicit positive integer
			{"0", complex(float32(0), 0)},           // Zero
			{"0.0", complex(float32(0), 0)},         // Zero with decimal
			{"+0", complex(float32(0), 0)},          // Explicit positive zero
			{"-0", complex(float32(0), 0)},          // Explicit negative zero
			// Scientific notation
			{"1e3", complex(float32(1000), 0)},    // Scientific notation
			{"1e-3", complex(float32(0.001), 0)},  // Small scientific notation
			{"1E3", complex(float32(1000), 0)},    // Uppercase E
			{"1e+3", complex(float32(1000), 0)},   // Explicit positive exponent
			{"1.5e2", complex(float32(150), 0)},   // Fractional with exponent
			{"-1.5e2", complex(float32(-150), 0)}, // Negative fractional with exponent
			// Imaginary numbers only
			{"1e3i", complex(float32(0), 1000)},   // Scientific notation imaginary
			{"1e-3i", complex(float32(0), 0.001)}, // Small scientific notation imaginary
			{"42i", complex(float32(0), 42)},      // Basic imaginary
			{"-42i", complex(float32(0), -42)},    // Negative imaginary
			{"+42i", complex(float32(0), 42)},     // Explicit positive imaginary
			{"0i", complex(float32(0), 0)},        // Zero imaginary
			{"1i", complex(float32(0), 1)},        // Unit imaginary
			{"-1i", complex(float32(0), -1)},      // Negative unit imaginary
			{"1.5i", complex(float32(0), 1.5)},    // Fractional imaginary
			// Scientific notation imaginary
			{"1E3i", complex(float32(0), 1000)},    // Uppercase E imaginary
			{"1e+3i", complex(float32(0), 1000)},   // Explicit positive exponent imaginary
			{"1.5e2i", complex(float32(0), 150)},   // Fractional with exponent imaginary
			{"-1.5e2i", complex(float32(0), -150)}, // Negative fractional with exponent imaginary
			// Complex with scientific notation
			{"1e3+2e2i", complex(float32(1000), float32(200))},     // Both parts scientific
			{"1e-3+2e-2i", complex(float32(0.001), float32(0.02))}, // Both parts small scientific
			{"1.5e2-2.5e1i", complex(float32(150), float32(-25))},  // Mixed signs with scientific
			// Decimal variations
			{".5", complex(float32(0.5), 0)},    // Leading decimal point
			{"-.5", complex(float32(-0.5), 0)},  // Negative leading decimal
			{"+.5", complex(float32(0.5), 0)},   // Positive leading decimal
			{"5.", complex(float32(5.0), 0)},    // Trailing decimal point
			{"-5.", complex(float32(-5.0), 0)},  // Negative trailing decimal
			{"+5.", complex(float32(5.0), 0)},   // Positive trailing decimal
			{".5i", complex(float32(0), 0.5)},   // Leading decimal imaginary
			{"-.5i", complex(float32(0), -0.5)}, // Negative leading decimal imaginary
			{"+.5i", complex(float32(0), 0.5)},  // Positive leading decimal imaginary
			{"5.i", complex(float32(0), 5.0)},   // Trailing decimal imaginary
			{"-5.i", complex(float32(0), -5.0)}, // Negative trailing decimal imaginary
			{"+5.i", complex(float32(0), 5.0)},  // Positive trailing decimal imaginary
			// Complex decimal variations
			{".5+.5i", complex(float32(0.5), float32(0.5))},  // Both parts leading decimal
			{"5.+5.i", complex(float32(5.0), float32(5.0))},  // Both parts trailing decimal
			{".5-.5i", complex(float32(0.5), float32(-0.5))}, // Leading decimal with negative
			// Special values - infinity
			{"inf", complex(float32(math.Inf(1)), 0)},                // Real infinity
			{"+inf", complex(float32(math.Inf(1)), 0)},               // Positive real infinity
			{"-inf", complex(float32(math.Inf(-1)), 0)},              // Negative real infinity
			{"Inf", complex(float32(math.Inf(1)), 0)},                // Capitalized infinity
			{"infinity", complex(float32(math.Inf(1)), 0)},           // Full word infinity
			{"Infinity", complex(float32(math.Inf(1)), 0)},           // Capitalized full word infinity
			{"infi", complex(float32(0), float32(math.Inf(1)))},      // Imaginary infinity
			{"+infi", complex(float32(0), float32(math.Inf(1)))},     // Positive imaginary infinity
			{"-infi", complex(float32(0), float32(math.Inf(-1)))},    // Negative imaginary infinity
			{"Infi", complex(float32(0), float32(math.Inf(1)))},      // Capitalized imaginary infinity
			{"infinityi", complex(float32(0), float32(math.Inf(1)))}, // Full word imaginary infinity
			{"Infinityi", complex(float32(0), float32(math.Inf(1)))}, // Capitalized full word imaginary infinity
			// Complex with special values
			{"inf+1i", complex(float32(math.Inf(1)), float32(1))},                // Real infinity with imaginary
			{"1+infi", complex(float32(1), float32(math.Inf(1)))},                // Real with imaginary infinity
			{"inf+infi", complex(float32(math.Inf(1)), float32(math.Inf(1)))},    // Both infinities
			{"-inf-infi", complex(float32(math.Inf(-1)), float32(math.Inf(-1)))}, // Both negative infinities
			// Parentheses format
			{"(42+42i)", complex(float32(42), float32(42))},    // Complex in parentheses
			{"(42)", complex(float32(42), float32(0))},         // Real in parentheses
			{"(42i)", complex(float32(0), float32(42))},        // Imaginary in parentheses
			{"(-42-42i)", complex(float32(-42), float32(-42))}, // Negative complex in parentheses
		},
		fail: []decodeHookFailureTestCase[string, complex64]{
			{strings.Repeat("42", 420)},
			{"42.42.42"},
			{"abc"},            // Non-numeric
			{""},               // Empty string
			{"42abc"},          // Trailing non-numeric
			{"abc42"},          // Leading non-numeric
			{"42+abc"},         // Invalid imaginary part
			{"abc+42i"},        // Invalid real part
			{"42++42i"},        // Double plus
			{"42+-+42i"},       // Multiple signs
			{"42+42j"},         // Wrong imaginary unit
			{"42+42k"},         // Wrong imaginary unit
			{"42 + 42i"},       // Spaces around operator
			{"42+42i+1"},       // Extra components
			{"42+42i+1i"},      // Multiple imaginary parts
			{"42+42i+1+2i"},    // Too many components
			{"(42+42i"},        // Unclosed parenthesis
			{"42+42i)"},        // Extra closing parenthesis
			{"((42+42i))"},     // Double parentheses
			{"(42+42i)(1+1i)"}, // Multiple complex numbers
			{"42i+42"},         // Imaginary first (not standard)
			{"i"},              // Just 'i'
			{"42.42.42+1i"},    // Invalid real part
			{"42+42.42.42i"},   // Invalid imaginary part
			{"1e"},             // Incomplete scientific notation
			{"1e+"},            // Incomplete scientific notation
			{"1e-"},            // Incomplete scientific notation
			{"1e+i"},           // Incomplete scientific notation
			{"1.2.3+1i"},       // Multiple dots in real
			{"1+1.2.3i"},       // Multiple dots in imaginary
			{"1..2+1i"},        // Double dots in real
			{"1+1..2i"},        // Double dots in imaginary
			{".+.i"},           // Just dots
			{"1e1e1+1i"},       // Multiple exponents in real
			{" 42+42i "},       // Whitespace not handled by strconv
			{"\t42i\n"},        // Whitespace not handled by strconv
			{"\r42\r"},         // Whitespace not handled by strconv
			{" 42+42i "},       // Whitespace not handled by strconv
			{"\t42i\n"},        // Whitespace not handled by strconv
			{"\r42\r"},         // Whitespace not handled by strconv
			{"1+1e1e1i"},       // Multiple exponents in imaginary
			{"∞"},              // Unicode infinity
			{"∞+∞i"},           // Unicode infinity complex
			{"NaΝ"},            // Unicode NaN lookalike
		},
	}

	// Custom test for NaN since NaN != NaN
	t.Run("NaN", func(t *testing.T) {
		f := StringToComplex64HookFunc()

		// Test real NaN
		actual, err := DecodeHookExec(f, reflect.ValueOf("nan"), reflect.ValueOf(complex64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		c := actual.(complex64)
		if !math.IsNaN(float64(real(c))) || imag(c) != 0 {
			t.Fatalf("expected NaN+0i, got %v", c)
		}

		// Test imaginary NaN
		actual, err = DecodeHookExec(f, reflect.ValueOf("nani"), reflect.ValueOf(complex64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		c = actual.(complex64)
		if real(c) != 0 || !math.IsNaN(float64(imag(c))) {
			t.Fatalf("expected 0+NaNi, got %v", c)
		}

		// Test complex NaN
		actual, err = DecodeHookExec(f, reflect.ValueOf("nan+nani"), reflect.ValueOf(complex64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		c = actual.(complex64)
		if !math.IsNaN(float64(real(c))) || !math.IsNaN(float64(imag(c))) {
			t.Fatalf("expected NaN+NaNi, got %v", c)
		}
	})

	suite.Run(t)
}

func TestStringToBoolHookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, bool]{
		fn: StringToBoolHookFunc(),
		ok: []decodeHookTestCase[string, bool]{
			// True values (only those accepted by strconv.ParseBool)
			{"true", true}, // Boolean true (lowercase)
			{"True", true}, // Boolean true (capitalized)
			{"TRUE", true}, // Boolean true (uppercase)
			{"t", true},    // Single character true (lowercase)
			{"T", true},    // Single character true (uppercase)
			{"1", true},    // Numeric true

			// False values (only those accepted by strconv.ParseBool)
			{"false", false}, // Boolean false (lowercase)
			{"False", false}, // Boolean false (capitalized)
			{"FALSE", false}, // Boolean false (uppercase)
			{"f", false},     // Single character false (lowercase)
			{"F", false},     // Single character false (uppercase)
			{"0", false},     // Numeric false
		},
		fail: []decodeHookFailureTestCase[string, bool]{
			{""},           // Empty string
			{"maybe"},      // Invalid boolean word
			{"yes"},        // Not accepted by strconv.ParseBool
			{"no"},         // Not accepted by strconv.ParseBool
			{"on"},         // Not accepted by strconv.ParseBool
			{"off"},        // Not accepted by strconv.ParseBool
			{"y"},          // Not accepted by strconv.ParseBool
			{"n"},          // Not accepted by strconv.ParseBool
			{"yes please"}, // Invalid boolean phrase
			{"true false"}, // Multiple boolean values
			{"2"},          // Invalid number (only 0/1 accepted)
			{"-1"},         // Negative number
			{"10"},         // Number greater than 1
			{"abc"},        // Non-boolean text
			{"True False"}, // Multiple boolean values (capitalized)
			{"1.0"},        // Float representation of 1
			{"0.0"},        // Float representation of 0
			{"++true"},     // Double positive prefix
			{"--false"},    // Double negative prefix
			{"truee"},      // Typo in true
			{"fasle"},      // Typo in false
			{"tru"},        // Incomplete true
			{"fals"},       // Incomplete false
			{" true "},     // Whitespace not handled by strconv.ParseBool
			{"\ttrue\n"},   // Tab and newline whitespace
			{"\rfalse\r"},  // Carriage return whitespace
			{" 1 "},        // Whitespace around numeric true
			{" 0 "},        // Whitespace around numeric false
			{"∞"},          // Unicode infinity symbol
			{"тrue"},       // Cyrillic lookalike characters
		},
	}

	// Test non-string and non-bool type passthrough
	t.Run("Passthrough", func(t *testing.T) {
		f := StringToBoolHookFunc()

		// Non-string type should pass through
		intValue := reflect.ValueOf(42)
		actual, err := DecodeHookExec(f, intValue, reflect.ValueOf(false))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if actual != 42 {
			t.Fatalf("expected 42, got %v", actual)
		}

		// Non-bool target type should pass through
		strValue := reflect.ValueOf("true")
		actual, err = DecodeHookExec(f, strValue, strValue)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if actual != "true" {
			t.Fatalf("expected 'true', got %v", actual)
		}
	})

	suite.Run(t)
}

func TestStringToComplex128HookFunc(t *testing.T) {
	suite := decodeHookTestSuite[string, complex128]{
		fn: StringToComplex128HookFunc(),
		ok: []decodeHookTestCase[string, complex128]{
			// Standard complex numbers
			{"42.42+42.42i", complex(42.42, 42.42)}, // Basic complex number
			{"1+2i", complex(1, 2)},                 // Simple complex number
			{"-1-2i", complex(-1, -2)},              // Negative real and imaginary
			{"1-2i", complex(1, -2)},                // Positive real, negative imaginary
			{"-1+2i", complex(-1, 2)},               // Negative real, positive imaginary
			// Real numbers only
			{"-42.42", complex(-42.42, 0)}, // Negative real number
			{"42", complex(42, 0)},         // Positive integer
			{"+42", complex(42, 0)},        // Explicit positive integer
			{"0", complex(0, 0)},           // Zero
			{"0.0", complex(0, 0)},         // Zero with decimal
			{"+0", complex(0, 0)},          // Explicit positive zero
			{"-0", complex(0, 0)},          // Explicit negative zero
			// Scientific notation
			{"1e3", complex(1000, 0)},                            // Scientific notation
			{"1e-3", complex(0.001, 0)},                          // Small scientific notation
			{"1E3", complex(1000, 0)},                            // Uppercase E
			{"1e+3", complex(1000, 0)},                           // Explicit positive exponent
			{"1.5e2", complex(150, 0)},                           // Fractional with exponent
			{"-1.5e2", complex(-150, 0)},                         // Negative fractional with exponent
			{"1e-15", complex(1e-15, 0)},                         // Very small scientific notation
			{"3.141592653589793", complex(3.141592653589793, 0)}, // Pi with high precision
			// Imaginary numbers only
			{"1e3i", complex(0, 1000)},   // Scientific notation imaginary
			{"1e-3i", complex(0, 0.001)}, // Small scientific notation imaginary
			{"42i", complex(0, 42)},      // Basic imaginary
			{"-42i", complex(0, -42)},    // Negative imaginary
			{"+42i", complex(0, 42)},     // Explicit positive imaginary
			{"0i", complex(0, 0)},        // Zero imaginary
			{"1i", complex(0, 1)},        // Unit imaginary
			{"-1i", complex(0, -1)},      // Negative unit imaginary
			{"1.5i", complex(0, 1.5)},    // Fractional imaginary
			// Scientific notation imaginary
			{"1E3i", complex(0, 1000)},    // Uppercase E imaginary
			{"1e+3i", complex(0, 1000)},   // Explicit positive exponent imaginary
			{"1.5e2i", complex(0, 150)},   // Fractional with exponent imaginary
			{"-1.5e2i", complex(0, -150)}, // Negative fractional with exponent imaginary
			{"1e-15i", complex(0, 1e-15)}, // Very small scientific notation imaginary
			// Complex with scientific notation
			{"1e3+2e2i", complex(1000, 200)},        // Both parts scientific
			{"1e-3+2e-2i", complex(0.001, 0.02)},    // Both parts small scientific
			{"1.5e2-2.5e1i", complex(150, -25)},     // Mixed signs with scientific
			{"1e-15+1e-15i", complex(1e-15, 1e-15)}, // Both parts very small scientific
			// Decimal variations
			{".5", complex(0.5, 0)},    // Leading decimal point
			{"-.5", complex(-0.5, 0)},  // Negative leading decimal
			{"+.5", complex(0.5, 0)},   // Positive leading decimal
			{"5.", complex(5.0, 0)},    // Trailing decimal point
			{"-5.", complex(-5.0, 0)},  // Negative trailing decimal
			{"+5.", complex(5.0, 0)},   // Positive trailing decimal
			{".5i", complex(0, 0.5)},   // Leading decimal imaginary
			{"-.5i", complex(0, -0.5)}, // Negative leading decimal imaginary
			{"+.5i", complex(0, 0.5)},  // Positive leading decimal imaginary
			{"5.i", complex(0, 5.0)},   // Trailing decimal imaginary
			{"-5.i", complex(0, -5.0)}, // Negative trailing decimal imaginary
			{"+5.i", complex(0, 5.0)},  // Positive trailing decimal imaginary
			// Complex decimal variations
			{".5+.5i", complex(0.5, 0.5)},  // Both parts leading decimal
			{"5.+5.i", complex(5.0, 5.0)},  // Both parts trailing decimal
			{".5-.5i", complex(0.5, -0.5)}, // Leading decimal with negative
			// Very small and large numbers
			{"2.2250738585072014e-308", complex(2.2250738585072014e-308, 0)},  // Near min positive real
			{"1.7976931348623157e+308", complex(1.7976931348623157e+308, 0)},  // Near max real
			{"4.9406564584124654e-324", complex(4.9406564584124654e-324, 0)},  // Min positive subnormal real
			{"2.2250738585072014e-308i", complex(0, 2.2250738585072014e-308)}, // Near min positive imaginary
			{"1.7976931348623157e+308i", complex(0, 1.7976931348623157e+308)}, // Near max imaginary
			{"4.9406564584124654e-324i", complex(0, 4.9406564584124654e-324)}, // Min positive subnormal imaginary
			// Special values - infinity
			{"inf", complex(math.Inf(1), 0)},       // Real infinity
			{"+inf", complex(math.Inf(1), 0)},      // Positive real infinity
			{"-inf", complex(math.Inf(-1), 0)},     // Negative real infinity
			{"Inf", complex(math.Inf(1), 0)},       // Capitalized infinity
			{"infinity", complex(math.Inf(1), 0)},  // Full word infinity
			{"Infinity", complex(math.Inf(1), 0)},  // Capitalized full word infinity
			{"infi", complex(0, math.Inf(1))},      // Imaginary infinity
			{"+infi", complex(0, math.Inf(1))},     // Positive imaginary infinity
			{"-infi", complex(0, math.Inf(-1))},    // Negative imaginary infinity
			{"Infi", complex(0, math.Inf(1))},      // Capitalized imaginary infinity
			{"infinityi", complex(0, math.Inf(1))}, // Full word imaginary infinity
			{"Infinityi", complex(0, math.Inf(1))}, // Capitalized full word imaginary infinity
			// Complex with special values
			{"inf+1i", complex(math.Inf(1), 1)},                // Real infinity with imaginary
			{"1+infi", complex(1, math.Inf(1))},                // Real with imaginary infinity
			{"inf+infi", complex(math.Inf(1), math.Inf(1))},    // Both infinities
			{"-inf-infi", complex(math.Inf(-1), math.Inf(-1))}, // Both negative infinities
			// Parentheses format
			{"(42+42i)", complex(42, 42)},    // Complex in parentheses
			{"(42)", complex(42, 0)},         // Real in parentheses
			{"(42i)", complex(0, 42)},        // Imaginary in parentheses
			{"(-42-42i)", complex(-42, -42)}, // Negative complex in parentheses
		},
		fail: []decodeHookFailureTestCase[string, complex128]{
			{strings.Repeat("42", 420)},
			{"42.42.42"},
			{"abc"},            // Non-numeric
			{""},               // Empty string
			{"42abc"},          // Trailing non-numeric
			{"abc42"},          // Leading non-numeric
			{"42+abc"},         // Invalid imaginary part
			{"abc+42i"},        // Invalid real part
			{"42++42i"},        // Double plus
			{"42+-+42i"},       // Multiple signs
			{"42+42j"},         // Wrong imaginary unit
			{"42+42k"},         // Wrong imaginary unit
			{"42 + 42i"},       // Spaces around operator
			{"42+42i+1"},       // Extra components
			{"42+42i+1i"},      // Multiple imaginary parts
			{"42+42i+1+2i"},    // Too many components
			{"(42+42i"},        // Unclosed parenthesis
			{"42+42i)"},        // Extra closing parenthesis
			{"((42+42i))"},     // Double parentheses
			{"(42+42i)(1+1i)"}, // Multiple complex numbers
			{"42i+42"},         // Imaginary first (not standard)
			{"i"},              // Just 'i'
			{"42.42.42+1i"},    // Invalid real part
			{"42+42.42.42i"},   // Invalid imaginary part
			{"1e"},             // Incomplete scientific notation
			{"1e+"},            // Incomplete scientific notation
			{"1e-"},            // Incomplete scientific notation
			{"1e+i"},           // Incomplete scientific notation
			{"1.2.3+1i"},       // Multiple dots in real
			{"1+1.2.3i"},       // Multiple dots in imaginary
			{"1..2+1i"},        // Double dots in real
			{"1+1..2i"},        // Double dots in imaginary
			{".+.i"},           // Just dots
			{"1e1e1+1i"},       // Multiple exponents in real
			{"1+1e1e1i"},       // Multiple exponents in imaginary
			{"∞"},              // Unicode infinity
			{"∞+∞i"},           // Unicode infinity complex
			{"NaΝ"},            // Unicode NaN lookalike
		},
	}

	// Custom test for NaN since NaN != NaN
	t.Run("NaN", func(t *testing.T) {
		f := StringToComplex128HookFunc()

		// Test real NaN
		actual, err := DecodeHookExec(f, reflect.ValueOf("nan"), reflect.ValueOf(complex128(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		c := actual.(complex128)
		if !math.IsNaN(real(c)) || imag(c) != 0 {
			t.Fatalf("expected NaN+0i, got %v", c)
		}

		// Test imaginary NaN
		actual, err = DecodeHookExec(f, reflect.ValueOf("nani"), reflect.ValueOf(complex128(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		c = actual.(complex128)
		if real(c) != 0 || !math.IsNaN(imag(c)) {
			t.Fatalf("expected 0+NaNi, got %v", c)
		}

		// Test complex NaN
		actual, err = DecodeHookExec(f, reflect.ValueOf("nan+nani"), reflect.ValueOf(complex128(0)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		c = actual.(complex128)
		if !math.IsNaN(real(c)) || !math.IsNaN(imag(c)) {
			t.Fatalf("expected NaN+NaNi, got %v", c)
		}
	})

	suite.Run(t)
}

func TestErrorLeakageDecodeHook(t *testing.T) {
	cases := []struct {
		value         any
		target        any
		hook          DecodeHookFunc
		allowNilError bool
	}{
		// case 0
		{1234, []string{}, StringToSliceHookFunc(","), true},
		{"testing", time.Second, StringToTimeDurationHookFunc(), false},
		{":testing", &url.URL{}, StringToURLHookFunc(), false},
		{"testing", net.IP{}, StringToIPHookFunc(), false},
		{"testing", net.IPNet{}, StringToIPNetHookFunc(), false},
		// case 5
		{"testing", time.Time{}, StringToTimeHookFunc(time.RFC3339), false},
		{"testing", time.Time{}, StringToTimeHookFunc(time.RFC3339), false},
		{true, true, WeaklyTypedHook, true},
		{true, "string", WeaklyTypedHook, true},
		{1.0, "string", WeaklyTypedHook, true},
		// case 10
		{1, "string", WeaklyTypedHook, true},
		{[]uint8{0x00}, "string", WeaklyTypedHook, true},
		{uint(0), "string", WeaklyTypedHook, true},
		{struct{}{}, struct{}{}, RecursiveStructToMapHookFunc(), true},
		{"testing", netip.Addr{}, StringToNetIPAddrHookFunc(), false},
		// case 15
		{"testing:testing", netip.AddrPort{}, StringToNetIPAddrPortHookFunc(), false},
		{"testing", netip.Prefix{}, StringToNetIPPrefixHookFunc(), false},
		{"testing", int8(0), StringToInt8HookFunc(), false},
		{"testing", uint8(0), StringToUint8HookFunc(), false},
		// case 20
		{"testing", int16(0), StringToInt16HookFunc(), false},
		{"testing", uint16(0), StringToUint16HookFunc(), false},
		{"testing", int32(0), StringToInt32HookFunc(), false},
		{"testing", uint32(0), StringToUint32HookFunc(), false},
		{"testing", int64(0), StringToInt64HookFunc(), false},
		// case 25
		{"testing", uint64(0), StringToUint64HookFunc(), false},
		{"testing", int(0), StringToIntHookFunc(), false},
		{"testing", uint(0), StringToUintHookFunc(), false},
		{"testing", float32(0), StringToFloat32HookFunc(), false},
		{"testing", float64(0), StringToFloat64HookFunc(), false},
		// case 30
		{"testing", true, StringToBoolHookFunc(), false},
		{"testing", byte(0), StringToByteHookFunc(), false},
		{"testing", rune(0), StringToRuneHookFunc(), false},
		{"testing", complex64(0), StringToComplex64HookFunc(), false},
		{"testing", complex128(0), StringToComplex128HookFunc(), false},
	}

	for i, tc := range cases {
		value := reflect.ValueOf(tc.value)
		target := reflect.ValueOf(tc.target)
		output, err := DecodeHookExec(tc.hook, value, target)

		if err == nil {
			if tc.allowNilError {
				continue
			}

			t.Fatalf("case %d: expected error from input %v:\n\toutput (%T): %#v\n\toutput (string): %v", i, tc.value, output, output, output)
		}

		strValue := fmt.Sprintf("%v", tc.value)
		if strings.Contains(err.Error(), strValue) {
			t.Errorf("case %d: error contains input value\n\terr: %v\n\tinput: %v", i, err, strValue)
		} else {
			t.Logf("case %d: got safe error: %v", i, err)
		}
	}
}
