package pflag

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func setUpTime(t *time.Time) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.TextVar(t, "time", time.Now(), "time stamp")
	return f
}

func TestText(t *testing.T) {
	testCases := []struct {
		input    string
		success  bool
		expected time.Time
	}{
		{"2003-01-02T15:04:05Z", true, time.Date(2003, 1, 2, 15, 04, 05, 0, time.UTC)},
		{"2003-01-02 15:05:01", false, time.Time{}}, //negative case, invalid layout
		{"2024-11-22T03:01:02Z", true, time.Date(2024, 11, 22, 3, 1, 02, 0, time.UTC)},
		{"2006-01-02T15:04:05+07:00", true, time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("UTC+7", 7*60*60))},
	}

	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull
	for i := range testCases {
		var ts time.Time
		f := setUpTime(&ts)
		tc := &testCases[i]
		arg := fmt.Sprintf("--time=%s", tc.input)
		err := f.Parse([]string{arg})
		if err != nil {
			if tc.success {
				t.Errorf("expected parsing to succeed, but got %q", err)
			}
			continue
		}
		if !tc.success {
			t.Errorf("expected parsing failure, but parsing succeeded")
			continue
		}
		parsedT := new(time.Time)
		err = f.GetText("time", parsedT)
		if err != nil {
			t.Errorf("Got error trying to fetch the time flag: %v", err)
		}
		if !parsedT.Equal(tc.expected) {
			t.Errorf("expected %q, got %q", tc.expected, parsedT)
		}

	}
}
