package uritemplate

import "testing"

var testEqualsCases = []struct {
	t1     string
	t2     string
	flags  CompareFlags
	equals bool
}{
	{"http://example.com/{foo}", "http://example.com/{foo}", 0, true},
	{"http://example.com/{foo}", "http://example.com/{bar}", 0, true},
	{"http://example.com/{foo}", "http://example.com/{bar}", CompareVarname, false},
	{"http://example.com/{foo:1}", "http://example.com/{foo:2}", 0, false},
}

func TestEquals(t *testing.T) {
	for _, c := range testEqualsCases {
		if Equals(MustNew(c.t1), MustNew(c.t2), c.flags) != c.equals {
			t.Errorf("unexpected result %v when comparing %q and %q", !c.equals, c.t1, c.t2)
		}
	}
}
