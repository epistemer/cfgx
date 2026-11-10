package engine

import (
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// roundTrip checks that parse reads format's text back to v.
func roundTrip[T any](t *testing.T, v T, format func(T) string, parse func(string, *T) error) {
	t.Helper()
	text := format(v)
	var got T
	if err := parse(text, &got); err != nil {
		t.Errorf("%v → %q: %v", v, text, err)
		return
	}
	if !reflect.DeepEqual(got, v) {
		t.Errorf("%v → %q → %v", v, text, got)
	}
}

func items[T any](parse func([]string, *T) error) func(string, *T) error {
	return func(raw string, dst *T) error {
		its, err := SplitCSV(raw)
		if err != nil {
			return err
		}
		return parse(its, dst)
	}
}

// Every kind's canonical text parses back to the same value.
func TestFormatRoundTrips(t *testing.T) {
	for _, b := range []bool{false, true} {
		roundTrip(t, b, FormatBool, ParseBool)
	}
	for _, n := range []int{0, -3, 1 << 40} {
		roundTrip(t, n, FormatInt, ParseInt)
	}
	roundTrip(t, int64(-1<<62), FormatInt64, ParseInt64)
	roundTrip(t, uint(1<<63), FormatUint, ParseUint)
	for _, f := range []float64{0, 0.1, -2.5e-300, 1e21} {
		roundTrip(t, f, FormatFloat, ParseFloat)
	}
	for _, d := range []time.Duration{0, time.Nanosecond, 90 * time.Minute, -time.Second} {
		roundTrip(t, d, FormatDuration, ParseDuration)
	}
	for _, n := range []int64{0, 1, 1500, 1 << 20, 3 << 50} {
		roundTrip(t, n, FormatByteSize, ParseByteSize)
	}
	u, _ := url.Parse("https://user:pa%20ss@example.com:8443/x?y=1#z")
	for _, u := range []*url.URL{nil, u} {
		roundTrip(t, u, FormatURL, ParseURL)
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no zone data:", err)
	}
	for _, l := range []*time.Location{nil, time.UTC, berlin} {
		roundTrip(t, l, FormatLocation, ParseLocation)
	}
	for _, s := range [][]string{nil, {""}, {"a", "b,c", ` d `, `say "hi"`, "two\nlines"}, {"", ""}} {
		roundTrip(t, s, func(s []string) string {
			text, err := FormatStrings(s)
			if err != nil {
				t.Fatal(err)
			}
			return text
		}, items(ParseStrings))
	}
	roundTrip(t, []int{1, -2}, FormatInts, items(ParseInts))
	roundTrip(t, []time.Duration{time.Second, 5 * time.Minute}, FormatDurations, items(ParseDurations))
	stringMap := func(m map[string]string) string {
		s, err := FormatStringMap(m)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, m := range []map[string]string{nil, {"a": "1", "b": "x=y", "c": "", "d,e": " f"}} {
		roundTrip(t, m, stringMap, items(ParseStringMap))
	}
	for _, m := range []map[string]string{{"a=b": "1"}, {"": "1"}, {" a": "1"}, {"a": "x\ry"}} {
		if _, err := FormatStringMap(m); err == nil {
			t.Errorf("%q formatted", m)
		}
	}
	if _, err := FormatStrings([]string{"a\r\nb"}); err == nil {
		t.Error("an item with a carriage return formatted")
	}

	type level int
	texts, values := []string{"low", "high"}, []level{1, 2}
	enum := func(v level) string {
		s, err := FormatEnum(v, texts, values)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	parseEnum := func(raw string, dst *level) error { return ParseEnum(raw, texts, values, dst) }
	for _, v := range []level{0, 2} {
		roundTrip(t, v, enum, parseEnum)
	}
	if _, err := FormatEnum(level(9), texts, values); err == nil {
		t.Error("an enum value outside the enum formatted")
	}

	text := func(p netip.Prefix) string {
		s, err := FormatText(p)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, p := range []netip.Prefix{{}, netip.MustParsePrefix("10.0.0.0/8")} {
		roundTrip(t, p, text, ParseText[netip.Prefix])
	}
	type plain struct{ X int }
	if _, err := FormatText(plain{1}); err == nil {
		t.Error("a type without MarshalText formatted")
	}
}

// A list's canonical text splits back into the same items.
func FuzzFormatStrings(f *testing.F) {
	for _, s := range []string{"a\x00b,c", "", "\x00", ` a `, `"q"`, "x\ny", "\r"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		in := strings.Split(s, "\x00")
		text, err := FormatStrings(in)
		if err != nil {
			return
		}
		var out []string
		if err := items(ParseStrings)(text, &out); err != nil {
			t.Fatalf("%q → %q: %v", in, text, err)
		}
		if !reflect.DeepEqual(out, in) {
			t.Fatalf("%q → %q → %q", in, text, out)
		}
	})
}

// A map's canonical text, when it has one, parses back to the same map.
func FuzzFormatStringMap(f *testing.F) {
	for _, s := range []string{"a\x001", "a\x00x=y\x00b\x00", " k\x00v", "\x00v", "a,b\x00\"c\""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		kv := strings.Split(s, "\x00")
		m := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		text, err := FormatStringMap(m)
		if err != nil {
			return
		}
		var out map[string]string
		if err := items(ParseStringMap)(text, &out); err != nil {
			t.Fatalf("%q → %q: %v", m, text, err)
		}
		if len(m) == 0 && len(out) == 0 {
			return
		}
		if !reflect.DeepEqual(out, m) {
			t.Fatalf("%q → %q → %q", m, text, out)
		}
	})
}
