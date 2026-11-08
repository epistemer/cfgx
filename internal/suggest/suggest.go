// Package suggest provides string similarity helpers for "did you mean" suggestions.
package suggest

import (
	"strings"
	"unicode/utf8"
)

// Closest returns the candidate most similar to name, if one is similar enough.
func Closest(name string, candidates []string) (string, bool) {
	if name == "" || len(candidates) == 0 {
		return "", false
	}

	// 1. A candidate equal to name is never suggested (it is not unknown). Skip it.
	// 2. If a candidate equals name ignoring case (strings.EqualFold), return it immediately (first such candidate in order).
	for _, c := range candidates {
		if c == "" || c == name {
			continue
		}
		if strings.EqualFold(c, name) {
			return c, true
		}
	}

	// 3. Otherwise compute Distance(name, c) for every candidate.
	n := utf8.RuneCountInString(name)
	limit := (n + 2) / 4
	if limit < 1 {
		limit = 1
	} else if limit > 4 {
		limit = 4
	}

	best := ""
	bestDist := limit + 1

	for _, c := range candidates {
		if c == "" || c == name {
			continue
		}
		d := Distance(name, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}

	// 4. Return the candidate with the smallest distance that is <= limit. Ties: the first in candidates order. None: "", false.
	if bestDist <= limit {
		return best, true
	}
	return "", false
}

// Distance returns the optimal string alignment distance between a and b, counted in runes: insertions, deletions, substitutions and transpositions of two adjacent runes each cost 1.
func Distance(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return utf8.RuneCountInString(b)
	}
	if b == "" {
		return utf8.RuneCountInString(a)
	}

	ra := []rune(a)
	rb := []rune(b)
	m := len(ra)
	n := len(rb)

	stride := n + 1
	d := make([]int, (m+1)*stride)

	for i := 0; i <= m; i++ {
		d[i*stride] = i
	}
	for j := 0; j <= n; j++ {
		d[j] = j
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}

			del := d[(i-1)*stride+j] + 1
			ins := d[i*stride+(j-1)] + 1
			sub := d[(i-1)*stride+(j-1)] + cost

			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}

			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				trans := d[(i-2)*stride+(j-2)] + 1
				if trans < min {
					min = trans
				}
			}

			d[i*stride+j] = min
		}
	}

	return d[m*stride+n]
}
