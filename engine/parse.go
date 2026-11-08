package engine

import (
	"fmt"
	"strconv"
	"strings"
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
