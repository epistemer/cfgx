package engine

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// SplitCSV splits raw, one CSV record, into items: a,b gives two, and an
// item holding a comma or a quote is quoted: "a,b",c. Spaces after a comma
// are dropped. Empty text gives no items, which is how --tag "" or an empty
// env var clears a list (A4).
func SplitCSV(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	items, err := r.Read()
	if err == nil {
		if _, err2 := r.Read(); err2 != io.EOF {
			return nil, fmt.Errorf("%q has more than one line; quote an item that holds a line break", raw)
		}
	}
	if err != nil {
		var pe *csv.ParseError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		return nil, fmt.Errorf("%q isn't a valid comma-separated list: %v", raw, err)
	}
	return items, nil
}

// parseList parses each item with parse into a new slice. An empty list is
// nil. Unless empty is allowed, an empty item is a problem: for numbers, it
// would otherwise quietly be 0.
func parseList[T any](items []string, empty bool, parse func(string, *T) error, dst *[]T) error {
	if len(items) == 0 {
		*dst = nil
		return nil
	}
	out := make([]T, len(items))
	for i, it := range items {
		if !empty && strings.TrimSpace(it) == "" {
			return fmt.Errorf("item %d is empty", i+1)
		}
		if err := parse(it, &out[i]); err != nil {
			return fmt.Errorf("item %d: %v", i+1, err)
		}
	}
	*dst = out
	return nil
}

// ParseStrings stores items as they are; empty items are kept.
func ParseStrings(items []string, dst *[]string) error {
	return parseList(items, true, ParseString, dst)
}

// ParseInts parses each item as a base-10 int.
func ParseInts(items []string, dst *[]int) error {
	return parseList(items, false, ParseInt, dst)
}

// ParseDurations parses each item as a duration with a unit.
func ParseDurations(items []string, dst *[]time.Duration) error {
	return parseList(items, false, ParseDuration, dst)
}

// ParseStringMap parses key=value items. Spaces around a key are dropped;
// the value is kept exactly and may hold more = signs. A later item with
// the same key wins. An empty list is nil.
func ParseStringMap(items []string, dst *map[string]string) error {
	if len(items) == 0 {
		*dst = nil
		return nil
	}
	m := make(map[string]string, len(items))
	for i, it := range items {
		k, v, ok := strings.Cut(it, "=")
		switch {
		case !ok:
			return fmt.Errorf("item %d: %q isn't key=value", i+1, it)
		case strings.TrimSpace(k) == "":
			return fmt.Errorf("item %d: %q has an empty key", i+1, it)
		}
		m[strings.TrimSpace(k)] = v
	}
	*dst = m
	return nil
}
