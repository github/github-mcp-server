package iso8601

import (
	"fmt"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	for _, input := range []string{
		// Fast path (20 bytes)
		"1987-12-16T23:45:12Z",
		"2006-01-02T15:04:05Z",
		"2000-02-29T23:59:59Z", // leap year
		"2020-02-29T23:59:59Z", // leap year
		"0000-01-01T00:00:00Z",
		"9999-12-31T23:59:59Z",

		// Fast path (24 bytes)
		"1987-12-16T23:45:12.123Z",
		"2006-01-02T15:04:05.123Z",
		"2000-02-29T23:59:59.123Z",
		"2020-02-29T23:59:59.123Z",
		"0000-01-01T00:00:00.000Z",
		"9999-12-31T23:59:59.999Z",

		// Fast path (30 bytes)
		"1987-12-16T23:45:12.123456789Z",
		"2006-01-02T15:04:05.123456789Z",
		"2000-02-29T23:59:59.123456789Z",
		"2020-02-29T23:59:59.123456789Z",
		"0000-01-01T00:00:00.000000000Z",
		"9999-12-31T23:59:59.999999999Z",

		// Slow path
		"2006-01-02T15:04:05.1Z",
		"2006-01-02T15:04:05.12Z",
		"2006-01-02T15:04:05.1234Z",
		"2006-01-02T15:04:05.12345Z",
		"2006-01-02T15:04:05.123456Z",
		"2006-01-02T15:04:05.1234567Z",
		"2006-01-02T15:04:05.12345678Z",
		"2021-10-16T07:55:07+10:00",
		"2021-10-16T07:55:07.1+10:00",
		"2021-10-16T07:55:07.12+10:00",
		"2021-10-16T07:55:07.123+10:00",
		"2021-10-16T07:55:07.1234+10:00",
		"2021-10-16T07:55:07.12345+10:00",
		"2021-10-16T07:55:07.123456+10:00",
		"2021-10-16T07:55:07.1234567+10:00",
		"2021-10-16T07:55:07.12345678+10:00",
		"2021-10-16T07:55:07.123456789+10:00",
		"2021-10-16T07:55:07-10:00",
		"2021-10-16T07:55:07.1-10:00",
		"2021-10-16T07:55:07.12-10:00",
		"2021-10-16T07:55:07.123-10:00",
		"2021-10-16T07:55:07.1234-10:00",
		"2021-10-16T07:55:07.12345-10:00",
		"2021-10-16T07:55:07.123456-10:00",
		"2021-10-16T07:55:07.1234567-10:00",
		"2021-10-16T07:55:07.12345678-10:00",
		"2021-10-16T07:55:07.123456789-10:00",
	} {
		t.Run(input, func(t *testing.T) {
			expect, err := time.Parse(time.RFC3339Nano, input)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := Parse(input)
			if err != nil {
				t.Error(err)
			} else if !actual.Equal(expect) {
				t.Errorf("unexpected time: %v vs expected %v", actual, expect)
			} else if actual.Location().String() != expect.Location().String() {
				t.Errorf("unexpected timezone: %v vs expected %v", actual.Location().String(), expect.Location().String())
			}
		})
	}

	// Check ~4M YYYY-MM-DD dates in 20 byte form.
	for year := range 10000 {
		for month := range 14 {
			for day := range 33 {
				input := fmt.Sprintf("%04d-%02d-%02dT12:34:56Z", year, month, day)
				expect, expectErr := time.Parse(time.RFC3339Nano, input)
				actual, actualErr := Parse(input)
				if (expectErr != nil) != (actualErr != nil) {
					t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
				} else if !actual.Equal(expect) {
					t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
				}
			}
		}
	}

	// Check ~4M YYYY-MM-DD dates in 24 byte form.
	for year := range 10000 {
		for month := range 14 {
			for day := range 33 {
				input := fmt.Sprintf("%04d-%02d-%02dT12:34:56.789Z", year, month, day)
				expect, expectErr := time.Parse(time.RFC3339Nano, input)
				actual, actualErr := Parse(input)
				if (expectErr != nil) != (actualErr != nil) {
					t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
				} else if !actual.Equal(expect) {
					t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
				}
			}
		}
	}

	// Check ~4M YYYY-MM-DD dates in 30 byte form.
	for year := range 10000 {
		for month := range 14 {
			for day := range 33 {
				input := fmt.Sprintf("%04d-%02d-%02dT12:34:56.123456789Z", year, month, day)
				expect, expectErr := time.Parse(time.RFC3339Nano, input)
				actual, actualErr := Parse(input)
				if (expectErr != nil) != (actualErr != nil) {
					t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
				} else if !actual.Equal(expect) {
					t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
				}
			}
		}
	}

	// Check all ~1M HH:MM:SS times in 20 byte form.
	for hour := range 100 {
		for minute := range 100 {
			for second := range 100 {
				input := fmt.Sprintf("2000-01-01T%02d:%02d:%02dZ", hour, minute, second)
				expect, expectErr := time.Parse(time.RFC3339Nano, input)
				actual, actualErr := Parse(input)
				if (expectErr != nil) != (actualErr != nil) {
					t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
				} else if !actual.Equal(expect) {
					t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
				}
			}
		}
	}

	// Check ~1M HH:MM:SS.MMM times in 24 byte form.
	for hour := range 100 {
		for minute := range 100 {
			for second := range 100 {
				input := fmt.Sprintf("2000-01-01T%02d:%02d:%02d.123Z", hour, minute, second)
				expect, expectErr := time.Parse(time.RFC3339Nano, input)
				actual, actualErr := Parse(input)
				if (expectErr != nil) != (actualErr != nil) {
					t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
				} else if !actual.Equal(expect) {
					t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
				}
			}
		}
	}

	// Check ~1M HH:MM:SS.MMM times in 30 byte form.
	for hour := range 100 {
		for minute := range 100 {
			for second := range 100 {
				input := fmt.Sprintf("2000-01-01T%02d:%02d:%02d.123456789Z", hour, minute, second)
				expect, expectErr := time.Parse(time.RFC3339Nano, input)
				actual, actualErr := Parse(input)
				if (expectErr != nil) != (actualErr != nil) {
					t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
				} else if !actual.Equal(expect) {
					t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
				}
			}
		}
	}

	// Check milliseconds.
	for millis := 1; millis < 1000; millis <<= 1 {
		input := fmt.Sprintf("2000-01-01T00:00:00.%03dZ", millis)
		expect, expectErr := time.Parse(time.RFC3339Nano, input)
		actual, actualErr := Parse(input)
		if (expectErr != nil) != (actualErr != nil) {
			t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
		} else if !actual.Equal(expect) {
			t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
		}
	}

	// Check nanoseconds.
	for nanos := 1; nanos < 1e9; nanos <<= 1 {
		input := fmt.Sprintf("2000-01-01T00:00:00.%09dZ", nanos)
		expect, expectErr := time.Parse(time.RFC3339Nano, input)
		actual, actualErr := Parse(input)
		if (expectErr != nil) != (actualErr != nil) {
			t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
		} else if !actual.Equal(expect) {
			t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
		}
	}

	// Check with trailing zeroes omitted.
	for n := 1; n < 1e9; n <<= 1 {
		input := fmt.Sprintf("2000-01-01T00:00:00.%dZ", n)
		expect, expectErr := time.Parse(time.RFC3339Nano, input)
		actual, actualErr := Parse(input)
		if (expectErr != nil) != (actualErr != nil) {
			t.Errorf("unexpected error for %v: %v vs. %v expected", input, actualErr, expectErr)
		} else if !actual.Equal(expect) {
			t.Errorf("unexpected time for %v: %v vs. %v expected", input, actual, expect)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, input := range []string{
		// 20 bytes
		"XXXXXXXXXXXXXXXXXXXX",
		"00000000000000000000",
		"1900-02-29T00:00:00Z", // 28 days in month (not a leap year)
		"2021-02-29T00:00:00Z", // 28 days in month (not a leap year)
		"2021-02-30T00:00:00Z", // 28 days in month
		"2021-02-31T00:00:00Z", // 28 days in month
		"2021-04-31T00:00:00Z", // 30 days in month
		"2021-06-31T00:00:00Z", // 30 days in month
		"2021-09-31T00:00:00Z", // 30 days in month
		"2021-11-31T00:00:00Z", // 30 days in month
		"XXXX-13-01T00:00:00Z", // invalid year
		"2000-13-01T00:00:00Z", // invalid month (1)
		"2000-00-01T00:00:00Z", // invalid month (2)
		"2000-XX-01T00:00:00Z", // invalid month (3)
		"2000-12-32T00:00:00Z", // invalid day (1)
		"2000-12-00T00:00:00Z", // invalid day (2)
		"2000-12-XXT00:00:00Z", // invalid day (3)
		"2000-12-31T24:00:00Z", // invalid hour (1)
		"2000-12-31TXX:00:00Z", // invalid hour (2)
		"2000-12-31T23:60:00Z", // invalid minute (1)
		"2000-12-31T23:XX:00Z", // invalid minute (2)
		"2000-12-31T23:59:60Z", // invalid second (1)
		"2000-12-31T23:59:XXZ", // invalid second (2)
		"1999-01-01 23:45:00Z", // missing T separator
		"1999 01-01T23:45:00Z", // missing date separator (1)
		"1999-01 01T23:45:00Z", // missing date separator (2)
		"1999-01-01T23 45:00Z", // missing time separator (1)
		"1999-01-01T23:45 00Z", // missing time separator (2)
		"1999-01-01T23:45:00 ", // missing timezone
		"1999-01-01t23:45:00Z", // lowercase T
		"1999-01-01T23:45:00z", // lowercase Z
		"X999-01-01T23:45:00Z", // X in various positions
		"1X99-01-01T23:45:00Z",
		"19X9-01-01T23:45:00Z",
		"199X-01-01T23:45:00Z",
		"1999X01-01T23:45:00Z",
		"1999-X1-01T23:45:00Z",
		"1999-0X-01T23:45:00Z",
		"1999-01X01T23:45:00Z",
		"1999-01-X1T23:45:00Z",
		"1999-01-0XT23:45:00Z",
		"1999-01-01X23:45:00Z",
		"1999-01-01TX3:45:00Z",
		"1999-01-01T2X:45:00Z",
		"1999-01-01T23X45:00Z",
		"1999-01-01T23:X5:00Z",
		"1999-01-01T23:4X:00Z",
		"1999-01-01T23:45X00Z",
		"1999-01-01T23:45:X0Z",
		"1999-01-01T23:45:0XZ",
		"1999-01-01T23:45:00X",

		// 24 bytes
		"XXXXXXXXXXXXXXXXXXXXXXXX",
		"000000000000000000000000",
		"1900-02-29T00:00:00.123Z", // 28 days in month (not a leap year)
		"2021-02-29T00:00:00.123Z", // 28 days in month (not a leap year)
		"2021-02-30T00:00:00.123Z", // 28 days in month
		"2021-02-31T00:00:00.123Z", // 28 days in month
		"2021-04-31T00:00:00.123Z", // 30 days in month
		"2021-06-31T00:00:00.123Z", // 30 days in month
		"2021-09-31T00:00:00.123Z", // 30 days in month
		"2021-11-31T00:00:00.123Z", // 30 days in month
		"XXXX-13-01T00:00:00.123Z", // invalid year
		"2000-13-01T00:00:00.123Z", // invalid month (1)
		"2000-00-01T00:00:00.123Z", // invalid month (2)
		"2000-XX-01T00:00:00.123Z", // invalid month (3)
		"2000-12-32T00:00:00.123Z", // invalid day (1)
		"2000-12-00T00:00:00.123Z", // invalid day (2)
		"2000-12-XXT00:00:00.123Z", // invalid day (3)
		"2000-12-31T24:00:00.123Z", // invalid hour (1)
		"2000-12-31TXX:00:00.123Z", // invalid hour (2)
		"2000-12-31T23:60:00.123Z", // invalid minute (1)
		"2000-12-31T23:XX:00.123Z", // invalid minute (2)
		"2000-12-31T23:59:60.123Z", // invalid second (1)
		"2000-12-31T23:59:XX.123Z", // invalid second (2)
		"2000-12-31T23:59:59.XXXZ", // invalid millis
		"1999-01-01 23:45:00.123Z", // missing T separator
		"1999 01-01T23:45:00.123Z", // missing date separator (1)
		"1999-01 01T23:45:00.123Z", // missing date separator (2)
		"1999-01-01T23 45:00.123Z", // missing time separator (1)
		"1999-01-01T23:45 00.123Z", // missing time separator (2)
		"1999-01-01T23:45:00 123Z", // missing time separator (3)
		"1999-01-01T23:45:00.123 ", // missing timezone
		"1999-01-01t23:45:00.123Z", // lowercase T
		"1999-01-01T23:45:00.123z", // lowercase Z
		"X999-01-01T23:45:00.123Z", // X in various positions
		"1X99-01-01T23:45:00.123Z",
		"19X9-01-01T23:45:00.123Z",
		"199X-01-01T23:45:00.123Z",
		"1999X01-01T23:45:00.123Z",
		"1999-X1-01T23:45:00.123Z",
		"1999-0X-01T23:45:00.123Z",
		"1999-01X01T23:45:00.123Z",
		"1999-01-X1T23:45:00.123Z",
		"1999-01-0XT23:45:00.123Z",
		"1999-01-01X23:45:00.123Z",
		"1999-01-01TX3:45:00.123Z",
		"1999-01-01T2X:45:00.123Z",
		"1999-01-01T23X45:00.123Z",
		"1999-01-01T23:X5:00.123Z",
		"1999-01-01T23:4X:00.123Z",
		"1999-01-01T23:45X00.123Z",
		"1999-01-01T23:45:X0.123Z",
		"1999-01-01T23:45:0X.123Z",
		"1999-01-01T23:45:00X123Z",
		"1999-01-01T23:45:00.X23Z",
		"1999-01-01T23:45:00.1X3Z",
		"1999-01-01T23:45:00.12XZ",
		"1999-01-01T23:45:00.123X",

		// 30 bytes
		"XXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
		"000000000000000000000000000000",
		"1900-02-29T00:00:00.123456789Z", // 28 days in month (not a leap year)
		"2021-02-29T00:00:00.123456789Z", // 28 days in month (not a leap year)
		"2021-02-30T00:00:00.123456789Z", // 28 days in month
		"2021-02-31T00:00:00.123456789Z", // 28 days in month
		"2021-04-31T00:00:00.123456789Z", // 30 days in month
		"2021-06-31T00:00:00.123456789Z", // 30 days in month
		"2021-09-31T00:00:00.123456789Z", // 30 days in month
		"2021-11-31T00:00:00.123456789Z", // 30 days in month
		"XXXX-13-01T00:00:00.123456789Z", // invalid year
		"2000-13-01T00:00:00.123456789Z", // invalid month (1)
		"2000-00-01T00:00:00.123456789Z", // invalid month (2)
		"2000-XX-01T00:00:00.123456789Z", // invalid month (3)
		"2000-12-32T00:00:00.123456789Z", // invalid day (1)
		"2000-12-00T00:00:00.123456789Z", // invalid day (2)
		"2000-12-XXT00:00:00.123456789Z", // invalid day (3)
		"2000-12-31T24:00:00.123456789Z", // invalid hour (1)
		"2000-12-31TXX:00:00.123456789Z", // invalid hour (2)
		"2000-12-31T23:60:00.123456789Z", // invalid minute (1)
		"2000-12-31T23:XX:00.123456789Z", // invalid minute (2)
		"2000-12-31T23:59:60.123456789Z", // invalid second (1)
		"2000-12-31T23:59:XX.123456789Z", // invalid second (2)
		"2000-12-31T23:59:59.XXXXXXXXXZ", // invalid nanos
		"1999-01-01 23:45:00.123456789Z", // missing T separator
		"1999 01-01T23:45:00.123456789Z", // missing date separator (1)
		"1999-01 01T23:45:00.123456789Z", // missing date separator (2)
		"1999-01-01T23 45:00.123456789Z", // missing time separator (1)
		"1999-01-01T23:45 00.123456789Z", // missing time separator (2)
		"1999-01-01T23:45:00 123456789Z", // missing time separator (3)
		"1999-01-01T23:45:00.123456789 ", // missing timezone
		"1999-01-01t23:45:00.123456789Z", // lowercase T
		"1999-01-01T23:45:00.123456789z", // lowercase Z
		"X999-01-01T23:45:00.123456789Z", // X in various positions
		"1X99-01-01T23:45:00.123456789Z",
		"19X9-01-01T23:45:00.123456789Z",
		"199X-01-01T23:45:00.123456789Z",
		"1999X01-01T23:45:00.123456789Z",
		"1999-X1-01T23:45:00.123456789Z",
		"1999-0X-01T23:45:00.123456789Z",
		"1999-01X01T23:45:00.123456789Z",
		"1999-01-X1T23:45:00.123456789Z",
		"1999-01-0XT23:45:00.123456789Z",
		"1999-01-01X23:45:00.123456789Z",
		"1999-01-01TX3:45:00.123456789Z",
		"1999-01-01T2X:45:00.123456789Z",
		"1999-01-01T23X45:00.123456789Z",
		"1999-01-01T23:X5:00.123456789Z",
		"1999-01-01T23:4X:00.123456789Z",
		"1999-01-01T23:45X00.123456789Z",
		"1999-01-01T23:45:X0.123456789Z",
		"1999-01-01T23:45:0X.123456789Z",
		"1999-01-01T23:45:00X123456789Z",
		"1999-01-01T23:45:00.X23456789Z",
		"1999-01-01T23:45:00.1X3456789Z",
		"1999-01-01T23:45:00.12X456789Z",
		"1999-01-01T23:45:00.123X56789Z",
		"1999-01-01T23:45:00.1234X6789Z",
		"1999-01-01T23:45:00.12345X789Z",
		"1999-01-01T23:45:00.123456X89Z",
		"1999-01-01T23:45:00.1234567X9Z",
		"1999-01-01T23:45:00.12345678XZ",
		"1999-01-01T23:45:00.123456789X",

		"2000-01-01T00:00:00.Z", // missing number after decimal point
	} {
		t.Run(input, func(t *testing.T) {
			ts, err := time.Parse(time.RFC3339Nano, input)
			if err == nil {
				t.Fatalf("expected time.Parse('%s') error, got %v", input, ts)
			}
			ts, actualErr := Parse(input)
			if (err != nil) != (actualErr != nil) {
				t.Fatalf("expected Parse('%s') error %v, got %v", input, err, actualErr)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	for range b.N {
		Parse("2006-01-02T15:04:05Z")
	}
}

func BenchmarkParseMilliseconds(b *testing.B) {
	for range b.N {
		Parse("2006-01-02T15:04:05.123Z")
	}
}

func BenchmarkParseMicroseconds(b *testing.B) {
	for range b.N {
		Parse("2006-01-02T15:04:05.123456Z")
	}
}

func BenchmarkParseNanoseconds(b *testing.B) {
	for range b.N {
		Parse("2006-01-02T15:04:05.123456789Z")
	}
}

func BenchmarkParseInvalid(b *testing.B) {
	for range b.N {
		Parse("2006-01-02T15:04:05.XZ")
	}
}
