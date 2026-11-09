package engine

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSplitCSV(t *testing.T) {
	for raw, want := range map[string][]string{
		"":             nil,
		"a":            {"a"},
		"a, b,c ":      {"a", "b", "c "},
		`"a,b",c`:      {"a,b", "c"},
		`""`:           {""},
		",":            {"", ""},
		`"say ""hi"""`: {`say "hi"`},
		`" lead"`:      {" lead"},
	} {
		got, err := SplitCSV(raw)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("SplitCSV(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for raw, want := range map[string]string{
		`"unclosed`: "isn't a valid comma-separated list",
		`a"b`:       "isn't a valid comma-separated list",
		"a\nb":      "has more than one line",
	} {
		if _, err := SplitCSV(raw); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("SplitCSV(%q) = %v; want %q", raw, err, want)
		}
	}
}

func TestParseLists(t *testing.T) {
	var ints []int
	if err := ParseInts([]string{"1", " 2 "}, &ints); err != nil || !reflect.DeepEqual(ints, []int{1, 2}) {
		t.Errorf("ParseInts = %v, %v", ints, err)
	}
	if err := ParseInts(nil, &ints); err != nil || ints != nil {
		t.Errorf("ParseInts(nil) = %v, %v", ints, err)
	}
	if err := ParseInts([]string{"1", ""}, &ints); err == nil || err.Error() != "item 2 is empty" {
		t.Errorf("empty int item: %v", err)
	}
	if err := ParseInts([]string{"1", "x"}, &ints); err == nil || err.Error() != `item 2: "x" is not an integer` {
		t.Errorf("bad int item: %v", err)
	}
	var ds []time.Duration
	if err := ParseDurations([]string{"5"}, &ds); err == nil || !strings.HasPrefix(err.Error(), "item 1: ") {
		t.Errorf("duration without unit: %v", err)
	}
	var ss []string
	if err := ParseStrings([]string{"", "a"}, &ss); err != nil || !reflect.DeepEqual(ss, []string{"", "a"}) {
		t.Errorf("ParseStrings = %q, %v", ss, err)
	}

	var m map[string]string
	if err := ParseStringMap([]string{" a =x=y", "b=", "a=z"}, &m); err != nil || !reflect.DeepEqual(m, map[string]string{"a": "z", "b": ""}) {
		t.Errorf("ParseStringMap = %v, %v", m, err)
	}
	for _, bad := range []string{"novalue", " =x"} {
		if err := ParseStringMap([]string{bad}, &m); err == nil || !strings.HasPrefix(err.Error(), "item 1: ") {
			t.Errorf("ParseStringMap(%q) = %v", bad, err)
		}
	}
}

// quoteAll writes items as one CSV record with every item quoted.
func quoteAll(items []string) string {
	q := make([]string, len(items))
	for i, it := range items {
		q[i] = `"` + strings.ReplaceAll(it, `"`, `""`) + `"`
	}
	return strings.Join(q, ",")
}

// SplitCSV never panics, and what it splits reads back the same once
// written as CSV: items survive quoting.
func FuzzSplitCSV(f *testing.F) {
	for _, s := range []string{"", "a,b", `"a,b",c`, `""`, `"x`, "a\nb", `"a""b"`, " a, b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		items, err := SplitCSV(raw)
		if err != nil || len(items) == 0 {
			return
		}
		for _, it := range items {
			if strings.Contains(it, "\r") {
				return // CSV reads a quoted \r\n as \n
			}
		}
		again, err := SplitCSV(quoteAll(items))
		if err != nil || !reflect.DeepEqual(again, items) {
			t.Fatalf("SplitCSV(%q) = %q; written back and split: %q, %v", raw, items, again, err)
		}
	})
}

// ParseStringMap never panics, and every key it keeps is non-empty, has no
// outer spaces, and came from an item.
func FuzzParseStringMap(f *testing.F) {
	for _, s := range []string{"a=1", "a=1\x00b=2", "=x", "a", " a =b=c", "a=1\x00a=2"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		items := strings.Split(s, "\x00")
		var m map[string]string
		if err := ParseStringMap(items, &m); err != nil {
			return
		}
		if len(m) > len(items) {
			t.Fatalf("%d keys from %d items", len(m), len(items))
		}
		for k := range m {
			if k == "" || strings.TrimSpace(k) != k {
				t.Fatalf("key %q from %q", k, items)
			}
		}
	})
}
