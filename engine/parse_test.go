package engine

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseNumbers(t *testing.T) {
	var i64 int64
	var u uint
	var f float64
	cases := []struct {
		name string
		run  func() error
		ok   bool
		want string // error substring, or the value
	}{
		{"int64", func() error { return ParseInt64(" 9223372036854775807 ", &i64) }, true, ""},
		{"int64 overflow", func() error { return ParseInt64("9223372036854775808", &i64) }, false, "out of range for an integer"},
		{"int64 junk", func() error { return ParseInt64("1e3", &i64) }, false, `"1e3" is not an integer`},
		{"uint", func() error { return ParseUint("42", &u) }, true, ""},
		{"uint negative", func() error { return ParseUint("-1", &u) }, false, "is not a non-negative integer"},
		{"uint plus minus", func() error { return ParseUint("+-1", &u) }, false, "is not a non-negative integer"},
		{"uint plus", func() error { return ParseUint("+42", &u) }, true, ""},
		{"float", func() error { return ParseFloat("1.5e3", &f) }, true, ""},
		{"float NaN", func() error { return ParseFloat("NaN", &f) }, false, "is not a finite number"},
		{"float Inf", func() error { return ParseFloat("+Inf", &f) }, false, "is not a finite number"},
		{"float junk", func() error { return ParseFloat("1,5", &f) }, false, `"1,5" is not a number`},
	}
	for _, c := range cases {
		err := c.run()
		if c.ok != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: got %v", c.name, err)
		}
	}
	if i64 != math.MaxInt64 || u != 42 || f != 1500 {
		t.Errorf("values: %d %d %g", i64, u, f)
	}
}

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{"": 0, "0": 0, "90s": 90 * time.Second, " 1h30m ": 90 * time.Minute, "1.5s": 1500 * time.Millisecond} {
		var d time.Duration
		if err := ParseDuration(in, &d); err != nil || d != want {
			t.Errorf("%q: got %v, %v", in, d, err)
		}
	}
	for in, want := range map[string]string{"30": `"30" has no unit`, "5 minutes": `"5 minutes" is not a duration`} {
		var d time.Duration
		if err := ParseDuration(in, &d); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v", in, err)
		}
	}
}

func TestParseByteSize(t *testing.T) {
	for in, want := range map[string]int64{"": 0, "0": 0, "512": 512, "512B": 512, "1KB": 1000, "1kib": 1024, "1 MiB": 1 << 20, " 2GB ": 2e9, "8EB": -1} {
		var n int64
		err := ParseByteSize(in, &n)
		if want == -1 {
			if err == nil {
				t.Errorf("%q: want an error", in)
			}
			continue
		}
		if err != nil || n != want {
			t.Errorf("%q: got %d, %v", in, n, err)
		}
	}
	for in, want := range map[string]string{"1.5MiB": "is not a size", "-1": "is not a size", "MiB": "is not a size", "9999999PiB": "out of range"} {
		var n int64
		if err := ParseByteSize(in, &n); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v", in, err)
		}
	}
	for n, want := range map[int64]string{0: "0", 1500: "1500", 1024: "1KiB", 1 << 20: "1MiB", 3 << 30: "3GiB"} {
		if got := FormatByteSize(n); got != want {
			t.Errorf("FormatByteSize(%d) = %q, want %q", n, got, want)
		}
	}
}

// FuzzParseByteSize: no panics, and every accepted size round-trips through
// FormatByteSize.
func FuzzParseByteSize(f *testing.F) {
	for _, s := range []string{"0", "512", "1KB", "1 MiB", "8PiB", "9223372036854775807", "-1", "1.5GB", "kb"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		var n int64
		if ParseByteSize(s, &n) != nil {
			return
		}
		if n < 0 {
			t.Fatalf("%q gave a negative size %d", s, n)
		}
		var back int64
		if err := ParseByteSize(FormatByteSize(n), &back); err != nil || back != n {
			t.Fatalf("%q: %d formats as %q, which parses as %d, %v", s, n, FormatByteSize(n), back, err)
		}
	})
}

// FuzzParseDuration: no panics, and every accepted duration round-trips
// through time.Duration.String.
func FuzzParseDuration(f *testing.F) {
	for _, s := range []string{"0", "90s", "1h30m", "1.5s", "30", "-5m", "9223372036854775807ns"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		var d time.Duration
		if ParseDuration(s, &d) != nil {
			return
		}
		var back time.Duration
		if err := ParseDuration(d.String(), &back); err != nil || back != d {
			t.Fatalf("%q: %v prints as %q, which parses as %v, %v", s, d, d.String(), back, err)
		}
	})
}

func TestParseEnum(t *testing.T) {
	type mode int
	texts, vals := []string{"sequential", "parallel"}, []mode{1, 2}
	for _, c := range []struct {
		raw  string
		want mode
		err  string
	}{
		{"parallel", 2, ""},
		{" sequential\n", 1, ""},
		{"", 0, ""},
		{"  ", 0, ""},
		{"Parallel", 0, `"Parallel" isn't one of sequential, parallel; did you mean parallel?`},
		{"fast", 0, `"fast" isn't one of sequential, parallel`},
	} {
		got := mode(9)
		err := ParseEnum(c.raw, texts, vals, &got)
		switch {
		case c.err != "" && (err == nil || err.Error() != c.err):
			t.Errorf("%q: err = %v, want %s", c.raw, err, c.err)
		case c.err == "" && (err != nil || got != c.want):
			t.Errorf("%q = %d, %v; want %d", c.raw, got, err, c.want)
		}
	}
}
