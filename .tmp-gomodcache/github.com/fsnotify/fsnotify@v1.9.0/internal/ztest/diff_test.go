package ztest

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func assertEqual(t *testing.T, a, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%v != %v", a, b)
	}
}

func splitChars(s string) []string {
	chars := make([]string, 0, len(s))
	// Assume ASCII inputs
	for i := 0; i != len(s); i++ {
		chars = append(chars, string(s[i]))
	}
	return chars
}

func rep(s string, count int) string {
	return strings.Repeat(s, count)
}

func TestGetOptCodes(t *testing.T) {
	a := "qabxcd"
	b := "abycdf"
	s := newMatcher(splitChars(a), splitChars(b))
	w := &bytes.Buffer{}
	for _, op := range s.GetOpCodes() {
		fmt.Fprintf(w, "%s a[%d:%d], (%s) b[%d:%d] (%s)\n", string(op.Tag),
			op.I1, op.I2, a[op.I1:op.I2], op.J1, op.J2, b[op.J1:op.J2])
	}
	result := w.String()
	expected := `d a[0:1], (q) b[0:0] ()
e a[1:3], (ab) b[0:2] (ab)
r a[3:4], (x) b[2:3] (y)
e a[4:6], (cd) b[3:5] (cd)
i a[6:6], () b[5:6] (f)
`
	if expected != result {
		t.Errorf("unexpected op codes: \n%s", result)
	}
}

func TestGroupedOpCodes(t *testing.T) {
	a := []string{}
	for i := 0; i != 39; i++ {
		a = append(a, fmt.Sprintf("%02d", i))
	}
	b := []string{}
	b = append(b, a[:8]...)
	b = append(b, " i")
	b = append(b, a[8:19]...)
	b = append(b, " x")
	b = append(b, a[20:22]...)
	b = append(b, a[27:34]...)
	b = append(b, " y")
	b = append(b, a[35:]...)
	s := newMatcher(a, b)
	w := &bytes.Buffer{}
	for _, g := range s.GetGroupedOpCodes(-1) {
		fmt.Fprintf(w, "group\n")
		for _, op := range g {
			fmt.Fprintf(w, "  %s, %d, %d, %d, %d\n", string(op.Tag),
				op.I1, op.I2, op.J1, op.J2)
		}
	}
	result := w.String()
	expected := `group
  e, 5, 8, 5, 8
  i, 8, 8, 8, 9
  e, 8, 11, 9, 12
group
  e, 16, 19, 17, 20
  r, 19, 20, 20, 21
  e, 20, 22, 21, 23
  d, 22, 27, 23, 23
  e, 27, 30, 23, 26
group
  e, 31, 34, 27, 30
  r, 34, 35, 30, 31
  e, 35, 38, 31, 34
`
	if expected != result {
		t.Errorf("unexpected op codes: \n%s", result)
	}
}

func TestWithAsciiOneInsert(t *testing.T) {
	sm := newMatcher(splitChars(rep("b", 100)),
		splitChars("a"+rep("b", 100)))
	assertEqual(t, sm.GetOpCodes(),
		[]opCode{{'i', 0, 0, 0, 1}, {'e', 0, 100, 1, 101}})

	sm = newMatcher(splitChars(rep("b", 100)),
		splitChars(rep("b", 50)+"a"+rep("b", 50)))
	assertEqual(t, sm.GetOpCodes(),
		[]opCode{{'e', 0, 50, 0, 50}, {'i', 50, 50, 50, 51}, {'e', 50, 100, 51, 101}})
}

func TestWithAsciiOnDelete(t *testing.T) {
	sm := newMatcher(splitChars(rep("a", 40)+"c"+rep("b", 40)),
		splitChars(rep("a", 40)+rep("b", 40)))
	assertEqual(t, sm.GetOpCodes(),
		[]opCode{{'e', 0, 40, 0, 40}, {'d', 40, 41, 40, 40}, {'e', 41, 81, 40, 80}})
}

func TestSFBugsComparingEmptyLists(t *testing.T) {
	groups := newMatcher(nil, nil).GetGroupedOpCodes(-1)
	assertEqual(t, len(groups), 0)
	result := Diff("", "")
	assertEqual(t, result, "")
}

func TestOutputFormatRangeFormatUnified(t *testing.T) {
	// Per the diff spec at http://www.unix.org/single_unix_specification/
	//
	// Each <range> field shall be of the form:
	//   %1d", <beginning line number>  if the range contains exactly one line,
	// and:
	//  "%1d,%1d", <beginning line number>, <number of lines> otherwise.
	// If a range is empty, its beginning line number shall be the number of
	// the line just before the range, or 0 if the empty range starts the file.
	fm := formatRangeUnified
	assertEqual(t, fm(3, 3), "3,0")
	assertEqual(t, fm(3, 4), "4")
	assertEqual(t, fm(3, 5), "4,2")
	assertEqual(t, fm(3, 6), "4,3")
	assertEqual(t, fm(0, 0), "0,0")
}

func TestDiffMatch(t *testing.T) {
	now := time.Now().UTC()
	year := fmt.Sprintf("%d", now.Year())

	tests := []struct {
		inGot, inWant, want string
	}{
		{"Hello", "Hello", ""},
		{"Hello", "He%(ANY)", ""},

		{"Hello " + year + "!", "Hello %(YEAR)!", ""},
		{"Hello " + year + "!", "Hello %(YEAR)", "\n--- have\n+++ want\n@@ -1 +1 @@\n-have Hello " + year + "!\n+want Hello " + year + "\n"},

		{"Hello xy", "Hello %(ANY 2)", ""},
		{"Hello xy", "Hello %(ANY 2,)", ""},
		{"Hello xy", "Hello %(ANY 2,4)", ""},

		{"Hello xy", "Hello %(ANY 3)", "\n--- have\n+++ want\n@@ -1 +1 @@\n-have Hello xy\n+want Hello .{3}?\n"},
		{"Hello xy", "Hello %(ANY ,1)", "\n--- have\n+++ want\n@@ -1 +1 @@\n-have Hello xy\n+want Hello .{,1}?\n"},

		{"Hello xy", "Hello%([a-z ]+)", ""},

		{"Hello 5xy", "Hello%([a-z ]+)", "\n--- have\n+++ want\n@@ -1 +1 @@\n-have Hello 5xy\n+want Hello[a-z ]+\n"},

		{
			`{
				"ID": 1,
				"SiteID": 1,
				"StartFromHitID": 0,
				"LastHitID": 3,
				"Path": "/tmp/goatcounter-export-test-20200630T00:25:05Z-0.csv.gz",
				"CreatedAt": "2020-06-30T00:25:05.855750823Z",
				"FinishedAt": null,
				"NumRows": 3,
				"Size": "0.0",
				"Hash": "sha256-7b756b6dd4d908eff7f7febad0fbdf59f2d7657d8fd09c8ff5133b45f86b1fbf",
				"Error": null
			}`,
			`{
				"ID": 1,
				"SiteID": 1,
				"StartFromHitID": 0,
				"LastHitID": 3,
				"Path": "/tmp/goatcounter-export-test-%(ANY)Z-0.csv.gz",
				"CreatedAt": "%(ANY)Z",
				"FinishedAt": null,
				"NumRows": 3,
				"Size": "0.0",
				"Hash": "sha256-%(ANY)",
				"Error": null
			}`,
			"",
		},

		{
			`{
				"ID": 1,
				"SiteID": 1,
				"StartFromHitID": 0,
				"LastHitID": 3,
				"Path": "/tmp/goatcounter-export-test-20200630T00:25:05Z-0.csv.gz",
				"CreatedAt": "2020-06-30T00:25:05.855750823Z",
				"FinishedAt": null,
				"NumRows": 5,
				"Size": "0.0",
				"Hash": "sha256-7b756b6dd4d908eff7f7febad0fbdf59f2d7657d8fd09c8ff5133b45f86b1fbf",
				"Error": null
			}`,
			`{
				"ID": 1,
				"SiteID": 1,
				"StartFromHitID": 0,
				"LastHitID": 3,
				"Path": "/tmp/goatcounter-export-test-%(ANY)Z-0.csv.gz",
				"CreatedAt": "%(ANY)T%(ANY)Z",
				"FinishedAt": null,
				"NumRows": 3,
				"Size": "0.0",
				"Hash": "sha256-%(ANY)",
				"Error": null
			}`,
			"\n--- have\n+++ want\n@@ -6,7 +6,7 @@\n      \"Path\": \"/tmp/goatcounter-export-test-20200630T00:25:05Z-0.csv.gz\",\n      \"CreatedAt\": \"2020-06-30T00:25:05.855750823Z\",\n      \"FinishedAt\": null,\n-have \"NumRows\": 5,\n+want \"NumRows\": 3,\n      \"Size\": \"0.0\",\n      \"Hash\": \"sha256-7b756b6dd4d908eff7f7febad0fbdf59f2d7657d8fd09c8ff5133b45f86b1fbf\",\n      \"Error\": null\n",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			tt.inGot = strings.ReplaceAll(tt.inGot, "\t", "")
			tt.inWant = strings.ReplaceAll(tt.inWant, "\t", "")

			got := DiffMatch(tt.inGot, tt.inWant)
			if got != tt.want {
				t.Errorf("\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestDiffJSON(t *testing.T) {
	tests := []struct {
		inHave, inWant, want string
	}{
		{`{}`, ``, ``},
		{``, `{}`, ``},

		{``, `{"x": "x"}`, `
--- have
+++ want
@@ -1 +1,3 @@
-have {}
+want {
+want     "x": "x"
+want }
`},

		{`[1]`, `[1]`, ``},
		{`"a"`, `"a"`, ``},
		{`{"a": "x"}`, `{  "a":   "x"}`, ``},

		{`{"a": "x"}`, `{  "a":   "y"}`, `
--- have
+++ want
@@ -1,3 +1,3 @@
      {
-have     "a": "x"
+want     "a": "y"
      }
`},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			tt.inHave = strings.ReplaceAll(tt.inHave, "\t", "")
			tt.inWant = strings.ReplaceAll(tt.inWant, "\t", "")

			have := Diff(tt.inHave, tt.inWant, DiffJSON)
			if have != tt.want {
				t.Errorf("\nhave: %q\nwant: %q", have, tt.want)
			}
		})
	}
}
