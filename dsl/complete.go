package dsl

import (
	"fmt"
	"strings"

	"github.com/epistemer/cfgx/internal/ir"
)

// Completion is how a flag's value completes in the shell: Files, Dirs or
// CompleteFunc. Enums complete their values without one.
type Completion struct {
	c        ir.Completion
	problems []string
}

// Files completes file names; with patterns such as "*.pem", only files
// with those extensions.
func Files(patterns ...string) Completion {
	var c Completion
	for _, p := range patterns {
		ext, ok := strings.CutPrefix(p, "*.")
		if !ok || ext == "" || strings.ContainsAny(ext, "*?[]/\\") {
			c.problems = append(c.problems, fmt.Sprintf("Files pattern %q must be *.ext, such as *.pem", p))
			continue
		}
		c.c.Exts = append(c.c.Exts, ext)
	}
	return c
}

// Dirs completes directory names.
func Dirs() Completion {
	return Completion{c: ir.Completion{Dirs: true}}
}

// CompleteFunc completes with f, which gets what has been typed so far and
// returns the candidates; the shell doesn't offer file names as well. f
// follows Validate's rules: a top-level exported function the generated
// code can import.
func CompleteFunc(f func(toComplete string) []string) Completion {
	return Completion{c: ir.Completion{Func: funcRef(f, caller(1))}}
}

// Complete sets how the key's flag completes in the shell.
func (k *key[K, T, G]) Complete(c Completion) K {
	k.problems = append(k.problems, c.problems...)
	cc := c.c
	k.complete = &cc
	return k.self
}
