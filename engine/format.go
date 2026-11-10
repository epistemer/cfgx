package engine

import (
	"encoding"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The Format helpers generated Format closures call: each writes a value as
// the canonical text its kind's parser reads back to the same value.

// FormatBool is "true" or "false".
func FormatBool(b bool) string { return strconv.FormatBool(b) }

// FormatInt is n in decimal.
func FormatInt(n int) string { return strconv.Itoa(n) }

// FormatInt64 is n in decimal.
func FormatInt64(n int64) string { return strconv.FormatInt(n, 10) }

// FormatUint is n in decimal.
func FormatUint(n uint) string { return strconv.FormatUint(uint64(n), 10) }

// FormatFloat is f in the shortest form that parses back exactly.
func FormatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// FormatDuration is d as Go writes it: 1m30s.
func FormatDuration(d time.Duration) string { return d.String() }

// FormatURL is the URL, password included; "" for none.
func FormatURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}

// FormatLocation is the zone's name; "" for none.
func FormatLocation(l *time.Location) string {
	if l == nil {
		return ""
	}
	return l.String()
}

// FormatStrings is one CSV record, items quoted where they need it.
func FormatStrings(s []string) string { return csvRecord(s, func(x string) string { return x }) }

// FormatInts is one CSV record.
func FormatInts(n []int) string { return csvRecord(n, strconv.Itoa) }

// FormatDurations is one CSV record.
func FormatDurations(d []time.Duration) string { return csvRecord(d, time.Duration.String) }

// FormatStringMap is one CSV record of key=value items, sorted by key.
func FormatStringMap(m map[string]string) string {
	keys := slices.Sorted(maps.Keys(m))
	return csvRecord(keys, func(k string) string { return k + "=" + m[k] })
}

// FormatEnum is the value's text; a value that isn't one of the enum's is
// a problem, unless it is T's zero value, which is "".
func FormatEnum[T comparable](v T, texts []string, values []T) (string, error) {
	for i, x := range values {
		if x == v {
			return texts[i], nil
		}
	}
	var zero T
	if v == zero {
		return "", nil // not set, which empty text means too
	}
	return "", fmt.Errorf("%v isn't one of %s", v, strings.Join(texts, ", "))
}

// FormatText is the value's MarshalText text; T's zero value is "".
func FormatText[T any](v T) (string, error) {
	if !NonZero(v) {
		return "", nil
	}
	m, ok := any(&v).(encoding.TextMarshaler)
	if !ok {
		return "", fmt.Errorf("%T doesn't implement encoding.TextMarshaler, so it can't be stored", v)
	}
	b, err := m.MarshalText()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// csvRecord writes items as one CSV record that SplitCSV splits back into
// the same items. A list of one empty item is "", quoted, since empty text
// is the empty list.
func csvRecord[T any](items []T, text func(T) string) string {
	out := make([]string, len(items))
	for i, it := range items {
		s := text(it)
		if s == "" || strings.ContainsAny(s, ",\"\r\n") || strings.TrimSpace(s) != s {
			s = `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
		}
		out[i] = s
	}
	return strings.Join(out, ",")
}
