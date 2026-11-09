package engine

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/suggest"
)

// The parsers generated Set closures call. Each one reads raw text from a
// flag, an env var or a config file; empty text is the zero value (A4).
// Messages quote the raw text, so secret kinds (M4.3) need parsers that
// don't.

// ParseString stores raw.
func ParseString(raw string, dst *string) error {
	*dst = raw
	return nil
}

// ParseInt parses a base-10 integer; empty means 0.
func ParseInt(raw string, dst *int) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, strconv.IntSize)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return fmt.Errorf("%q is out of range for an integer", raw)
		}
		return fmt.Errorf("%q is not an integer", raw)
	}
	*dst = int(n)
	return nil
}

// ParseBool parses true or false (and the other forms strconv accepts);
// empty means false.
func ParseBool(raw string, dst *bool) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = false
		return nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("%q is not a boolean; use true or false", raw)
	}
	*dst = b
	return nil
}

// ParseInt64 parses a base-10 int64; empty means 0.
func ParseInt64(raw string, dst *int64) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return numErr(raw, err, "an integer")
	}
	*dst = n
	return nil
}

// ParseUint parses a base-10 unsigned integer; empty means 0.
func ParseUint(raw string, dst *uint) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = 0
		return nil
	}
	// strconv.ParseUint rejects a sign; accept "+" as the other parsers do.
	n, err := strconv.ParseUint(strings.TrimPrefix(s, "+"), 10, strconv.IntSize)
	if err != nil {
		return numErr(raw, err, "a non-negative integer")
	}
	*dst = uint(n)
	return nil
}

// ParseFloat parses a float64; empty means 0. NaN and infinities are
// rejected: a setting needs a number.
func ParseFloat(raw string, dst *float64) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return numErr(raw, err, "a number")
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("%q is not a finite number", raw)
	}
	*dst = f
	return nil
}

func numErr(raw string, err error, what string) error {
	if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
		return fmt.Errorf("%q is out of range for %s", raw, what)
	}
	return fmt.Errorf("%q is not %s", raw, what)
}

// ParseDuration parses a duration such as 90s, 5m or 1h30m; empty means 0.
// A bare number other than 0 is rejected, so 30 can't mean 30ns by mistake.
func ParseDuration(raw string, dst *time.Duration) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = 0
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		if _, numErr := strconv.ParseFloat(s, 64); numErr == nil {
			return fmt.Errorf("%q has no unit; write it like 90s, 5m or 1h30m", raw)
		}
		return fmt.Errorf("%q is not a duration; write it like 90s, 5m or 1h30m", raw)
	}
	*dst = d
	return nil
}

// byteUnits are the size units, lower-cased: 1000s and 1024s.
var byteUnits = map[string]int64{
	"": 1, "b": 1,
	"kb": 1e3, "mb": 1e6, "gb": 1e9, "tb": 1e12, "pb": 1e15,
	"kib": 1 << 10, "mib": 1 << 20, "gib": 1 << 30, "tib": 1 << 40, "pib": 1 << 50,
}

// ParseByteSize parses a size: bytes, or a whole number with a unit, such as
// 512KB (1000s) or 1MiB (1024s). Units ignore case, and a space may separate
// them from the number. Empty means 0.
func ParseByteSize(raw string, dst *int64) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = 0
		return nil
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	unit, ok := byteUnits[strings.ToLower(strings.TrimSpace(s[i:]))]
	if i == 0 || !ok {
		return fmt.Errorf("%q is not a size; write bytes or a whole number with a unit, like 512KB or 1MiB", raw)
	}
	n, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil || n > math.MaxInt64/unit {
		return fmt.Errorf("%q is out of range for a size", raw)
	}
	*dst = n * unit
	return nil
}

// FormatByteSize writes n in the largest 1024s unit that divides it exactly,
// or in bytes: 1048576 is 1MiB, 1500 is 1500.
func FormatByteSize(n int64) string {
	for _, u := range []struct {
		name string
		size int64
	}{{"PiB", 1 << 50}, {"TiB", 1 << 40}, {"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}} {
		if n != 0 && n%u.size == 0 {
			return strconv.FormatInt(n/u.size, 10) + u.name
		}
	}
	return strconv.FormatInt(n, 10)
}

// ParseEnum stores the value whose text is raw: values[i] for texts[i].
// Matching is exact, after trimming spaces; empty means the zero value. A
// near miss gets a suggestion.
func ParseEnum[T comparable](raw string, texts []string, values []T, dst *T) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		var zero T
		*dst = zero
		return nil
	}
	for i, t := range texts {
		if t == s {
			*dst = values[i]
			return nil
		}
	}
	msg := fmt.Sprintf("%q isn't one of %s", raw, strings.Join(texts, ", "))
	if near, ok := suggest.Closest(s, texts); ok {
		msg += fmt.Sprintf("; did you mean %s?", near)
	}
	return errors.New(msg)
}

// ParseSecret stores raw as a secret, exactly; empty means unset. It never
// fails, so no message can quote the value.
func ParseSecret(raw string, dst *cfgx.Secret) error {
	*dst = cfgx.NewSecret(raw)
	return nil
}
