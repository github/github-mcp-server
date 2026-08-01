package assert

import (
	"fmt"
	"testing"
)

type Data struct {
	Label string
	Value int64
}

func TestBadMessage(t *testing.T) {
	invalidMessage := func() { True(t, false, 1234) }
	assertOk(t, "Non-fmt message value", func(t testing.TB) {
		Panics(t, invalidMessage)
	})
	assertFail(t, "Non-fmt message value", func(t testing.TB) {
		True(t, false, "example %s", "message")
	})
}

func TestTrue(t *testing.T) {
	assertOk(t, "Succeed", func(t testing.TB) {
		True(t, 1 > 0)
	})
	assertFail(t, "Fail", func(t testing.TB) {
		True(t, 1 < 0)
	})
}

func TestFalse(t *testing.T) {
	assertOk(t, "Succeed", func(t testing.TB) {
		False(t, 1 < 0)
	})
	assertFail(t, "Fail", func(t testing.TB) {
		False(t, 1 > 0)
	})
}

func TestEqual(t *testing.T) {
	assertOk(t, "Nil", func(t testing.TB) {
		Equal(t, interface{}(nil), interface{}(nil))
	})
	assertOk(t, "Identical structs", func(t testing.TB) {
		Equal(t, Data{"expected", 1234}, Data{"expected", 1234})
	})
	assertFail(t, "Different structs", func(t testing.TB) {
		Equal(t, Data{"expected", 1234}, Data{"actual", 1234})
	})
	assertOk(t, "Identical numbers", func(t testing.TB) {
		Equal(t, 1234, 1234)
	})
	assertFail(t, "Identical numbers", func(t testing.TB) {
		Equal(t, 1234, 1324)
	})
	assertOk(t, "Zero-length byte arrays", func(t testing.TB) {
		Equal(t, []byte(nil), []byte(""))
	})
	assertOk(t, "Identical byte arrays", func(t testing.TB) {
		Equal(t, []byte{1, 2, 3, 4}, []byte{1, 2, 3, 4})
	})
	assertFail(t, "Different byte arrays", func(t testing.TB) {
		Equal(t, []byte{1, 2, 3, 4}, []byte{1, 3, 2, 4})
	})
	assertOk(t, "Identical strings", func(t testing.TB) {
		Equal(t, "example", "example")
	})
	assertFail(t, "Identical strings", func(t testing.TB) {
		Equal(t, "example", "elpmaxe")
	})
}

func TestError(t *testing.T) {
	assertOk(t, "Error", func(t testing.TB) {
		Error(t, fmt.Errorf("example"))
	})
	assertFail(t, "Nil", func(t testing.TB) {
		Error(t, nil)
	})
}

func TestNoError(t *testing.T) {
	assertFail(t, "Error", func(t testing.TB) {
		NoError(t, fmt.Errorf("example"))
	})
	assertOk(t, "Nil", func(t testing.TB) {
		NoError(t, nil)
	})
}

func TestPanics(t *testing.T) {
	willPanic := func() { panic("example") }
	wontPanic := func() {}
	assertOk(t, "Will panic", func(t testing.TB) {
		Panics(t, willPanic)
	})
	assertFail(t, "Won't panic", func(t testing.TB) {
		Panics(t, wontPanic)
	})
}

func TestZero(t *testing.T) {
	assertOk(t, "Empty struct", func(t testing.TB) {
		Zero(t, Data{})
	})
	assertFail(t, "Non-empty struct", func(t testing.TB) {
		Zero(t, Data{Label: "example"})
	})
	assertOk(t, "Nil slice", func(t testing.TB) {
		var slice []int
		Zero(t, slice)
	})
	assertFail(t, "Non-empty slice", func(t testing.TB) {
		slice := []int{1, 2, 3, 4}
		Zero(t, slice)
	})
	assertOk(t, "Zero-length slice", func(t testing.TB) {
		slice := []int{}
		Zero(t, slice)
	})
}

func TestNotZero(t *testing.T) {
	assertFail(t, "Empty struct", func(t testing.TB) {
		zero := Data{}
		NotZero(t, zero)
	})
	assertOk(t, "Non-empty struct", func(t testing.TB) {
		notZero := Data{Label: "example"}
		NotZero(t, notZero)
	})
	assertFail(t, "Nil slice", func(t testing.TB) {
		var slice []int
		NotZero(t, slice)
	})
	assertFail(t, "Zero-length slice", func(t testing.TB) {
		slice := []int{}
		NotZero(t, slice)
	})
	assertOk(t, "Non-empty slice", func(t testing.TB) {
		slice := []int{1, 2, 3, 4}
		NotZero(t, slice)
	})
}

type testCase struct {
	*testing.T
	failed string
}

func (t *testCase) Fatal(args ...interface{}) {
	t.failed = fmt.Sprint(args...)
}

func (t *testCase) Fatalf(message string, args ...interface{}) {
	t.failed = fmt.Sprintf(message, args...)
}

func assertFail(t *testing.T, name string, fn func(t testing.TB)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		test := &testCase{T: t}
		fn(test)
		if test.failed == "" {
			t.Fatal("Test expected to fail but did not")
		} else {
			t.Log(test.failed)
		}
	})
}

func assertOk(t *testing.T, name string, fn func(t testing.TB)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		test := &testCase{T: t}
		fn(test)
		if test.failed != "" {
			t.Fatal("Test expected to succeed but did not:\n", test.failed)
		}
	})
}
