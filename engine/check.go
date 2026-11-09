package engine

import (
	"cmp"
	"fmt"
	"regexp"
	"unicode/utf8"
)

// The constraints generated Check closures call. Each says what's wrong
// without quoting the value: the problem names the key and the value's
// source, and the value may be a secret.

// AtLeast checks v >= min; text is min as --help would show it.
func AtLeast[T cmp.Ordered](v, min T, text string) error {
	if v < min {
		return fmt.Errorf("must be at least %s", text)
	}
	return nil
}

// AtMost checks v <= max; text is max as --help would show it.
func AtMost[T cmp.Ordered](v, max T, text string) error {
	if v > max {
		return fmt.Errorf("must be at most %s", text)
	}
	return nil
}

// Length checks that s is min to max characters (runes) long.
func Length(s string, min, max int) error {
	n := utf8.RuneCountInString(s)
	if n >= min && n <= max {
		return nil
	}
	if min == max {
		return fmt.Errorf("must be %d characters long, not %d", min, n)
	}
	return fmt.Errorf("must be %d to %d characters long, not %d", min, max, n)
}

// Match checks that re, compiled from ^(?:pattern)$, matches s.
func Match(s string, re *regexp.Regexp, pattern string) error {
	if !re.MatchString(s) {
		return fmt.Errorf("must match the pattern %s", pattern)
	}
	return nil
}
