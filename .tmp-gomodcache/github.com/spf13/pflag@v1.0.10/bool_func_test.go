package pflag

import (
	"strings"
	"testing"
)

func TestBoolFunc(t *testing.T) {
	var count int
	fn := func(_ string) error {
		count++
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.BoolFunc("func", "Callback function", fn)

	err := fset.Parse([]string{"--func", "--func=1", "--func=false"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 calls to the callback, got %d calls", count)
	}
}

func TestBoolFuncP(t *testing.T) {
	var count int
	fn := func(_ string) error {
		count++
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.BoolFuncP("bfunc", "b", "Callback function", fn)

	err := fset.Parse([]string{"--bfunc", "--bfunc=0", "--bfunc=false", "-b", "-b=0"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if count != 5 {
		t.Fatalf("expected 5 calls to the callback, got %d calls", count)
	}
}

func TestBoolFuncUsage(t *testing.T) {
	t.Run("regular func flag", func(t *testing.T) {
		// regular boolfunc flag:
		// expect to see '--flag1' followed by the usageMessage, and no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.BoolFunc("flag1", "usage message", func(s string) error { return nil })
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag1   usage message"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})

	t.Run("func flag with placeholder name", func(t *testing.T) {
		// func flag, with a placeholder name:
		// if usageMesage contains a placeholder, expect '--flag2 {placeholder}'; still expect no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.BoolFunc("flag2", "usage message with `name` placeholder", func(s string) error { return nil })
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag2 name   usage message with name placeholder"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})
}
