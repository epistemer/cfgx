package engine

import (
	"net/netip"
	"net/url"
	"reflect"
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
		roundTrip(t, s, FormatStrings, items(ParseStrings))
	}
	roundTrip(t, []int{1, -2}, FormatInts, items(ParseInts))
	roundTrip(t, []time.Duration{time.Second, 5 * time.Minute}, FormatDurations, items(ParseDurations))
	for _, m := range []map[string]string{nil, {"a": "1", "b": "x=y", "c": "", "d,e": " f"}} {
		roundTrip(t, m, FormatStringMap, items(ParseStringMap))
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
