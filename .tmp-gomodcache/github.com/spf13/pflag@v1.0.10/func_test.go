package pflag

import (
	"strings"
	"testing"
)

func cmpLists(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFunc(t *testing.T) {
	var values []string
	fn := func(s string) error {
		values = append(values, s)
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.Func("fnflag", "Callback function", fn)

	err := fset.Parse([]string{"--fnflag=aa", "--fnflag", "bb"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	expected := []string{"aa", "bb"}
	if !cmpLists(expected, values) {
		t.Fatalf("expected %v, got %v", expected, values)
	}
}

func TestFuncP(t *testing.T) {
	var values []string
	fn := func(s string) error {
		values = append(values, s)
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.FuncP("fnflag", "f", "Callback function", fn)

	err := fset.Parse([]string{"--fnflag=a", "--fnflag", "b", "-fc", "-f=d", "-f", "e"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	expected := []string{"a", "b", "c", "d", "e"}
	if !cmpLists(expected, values) {
		t.Fatalf("expected %v, got %v", expected, values)
	}
}

func TestFuncUsage(t *testing.T) {
	t.Run("regular func flag", func(t *testing.T) {
		// regular func flag:
		// expect to see '--flag1 value' followed by the usageMessage, and no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.Func("flag1", "usage message", func(s string) error { return nil })
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag1 value   usage message"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})

	t.Run("func flag with placeholder name", func(t *testing.T) {
		// func flag, with a placeholder name:
		// if usageMesage contains a placeholder, expect that name; still expect no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.Func("flag2", "usage message with `name` placeholder", func(s string) error { return nil })
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag2 name   usage message with name placeholder"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})
}
