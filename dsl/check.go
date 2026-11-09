package dsl

import (
	"cmp"
	"fmt"
	"go/token"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/epistemer/cfgx/internal/ir"
)

// Validate adds a check the loaded value must pass: f, a top-level
// exported function in a package the generated code can import (not main,
// the schema package, or a package that imports cfgx/dsl). Closures and
// methods are rejected when generating. f runs after the key's own
// constraints, only when the key is set and relevant; its error is reported
// with the value's source.
func (k *key[K, T, G]) Validate(f func(G) error) K {
	k.validate = funcRef(f, caller(1))
	return k.self
}

// DefaultFunc computes the default when loading, by calling f, a top-level
// exported function under the same rules as Validate's, such as
// runtime.NumCPU. A key has Default or DefaultFunc, not both.
func (k *key[K, T, G]) DefaultFunc(f func() G) K {
	k.defFunc = funcRef(f, caller(1))
	switch k.kind {
	case ir.KindSecretFile, ir.KindFile:
		k.defFunc.Problem = "a file key's default is a path; use Default"
	}
	return k.self
}

// funcRef names f by runtime.FuncForPC, under ADR 0004's rules.
func funcRef(f any, pos ir.Pos) *ir.FuncRef {
	ref := &ir.FuncRef{Pos: pos}
	v := reflect.ValueOf(f)
	if v.IsNil() {
		ref.Problem = "the function is nil"
		return ref
	}
	rf := runtime.FuncForPC(v.Pointer())
	if rf == nil {
		ref.Problem = "can't name the function"
		return ref
	}
	name := rf.Name()
	slash := strings.LastIndex(name, "/") + 1
	dot := strings.Index(name[slash:], ".")
	if dot < 0 {
		ref.Problem = fmt.Sprintf("can't name the function %s", name)
		return ref
	}
	ref.Path = strings.ReplaceAll(name[:slash+dot], "%2e", ".")
	ref.Name = name[slash+dot+1:]
	sym := ref.Name
	switch {
	case strings.Contains(sym, "["):
		ref.Problem = fmt.Sprintf("%s is a generic function; pass a non-generic top-level function", name)
	case strings.HasSuffix(sym, "-fm") || strings.HasPrefix(sym, "(") || (strings.Contains(sym, ".") && !strings.Contains(sym, ".func")):
		ref.Problem = fmt.Sprintf("%s is a method; pass a top-level function", name)
	case strings.Contains(sym, "."):
		ref.Problem = fmt.Sprintf("%s is a closure; pass a top-level function", name)
	case !token.IsIdentifier(sym) || !token.IsExported(sym):
		ref.Problem = fmt.Sprintf("%s isn't exported, so the generated code can't call it", name)
	}
	return ref
}

// ordered is a key whose values have an order: numbers and durations. Its
// constraints compare the value; they apply only when the key is set.
type ordered[K any, T cmp.Ordered, G any] struct {
	key[K, T, G]
}

// Range requires the value, when set, to be from min to max, inclusive.
func (k *ordered[K, T, G]) Range(min, max T) K {
	if min > max {
		k.problems = append(k.problems, fmt.Sprintf("Range(%s, %s): the minimum is above the maximum", k.text(min), k.text(max)))
		return k.self
	}
	k.Min(min)
	return k.Max(max)
}

// Min requires the value, when set, to be at least min.
func (k *ordered[K, T, G]) Min(min T) K {
	k.checks = append(k.checks, ir.Check{Kind: ir.CheckMin, Args: []string{k.text(min)}})
	k.finish = append(k.finish, func() string {
		if k.dv != nil && *k.dv < min {
			return fmt.Sprintf("default %s is below the minimum %s", k.text(*k.dv), k.text(min))
		}
		return ""
	})
	return k.self
}

// Max requires the value, when set, to be at most max.
func (k *ordered[K, T, G]) Max(max T) K {
	k.checks = append(k.checks, ir.Check{Kind: ir.CheckMax, Args: []string{k.text(max)}})
	k.finish = append(k.finish, func() string {
		if k.dv != nil && *k.dv > max {
			return fmt.Sprintf("default %s is above the maximum %s", k.text(*k.dv), k.text(max))
		}
		return ""
	})
	return k.self
}

// Len requires the value, when set, to be from min to max characters
// (runes) long.
func (k *StringKey) Len(min, max int) *StringKey {
	if min < 0 || min > max {
		k.problems = append(k.problems, fmt.Sprintf("Len(%d, %d) needs 0 <= min <= max", min, max))
		return k
	}
	k.checks = append(k.checks, ir.Check{Kind: ir.CheckLen, Args: []string{fmt.Sprint(min), fmt.Sprint(max)}})
	k.finish = append(k.finish, func() string {
		if k.dv != nil && *k.dv != "" {
			if n := utf8.RuneCountInString(*k.dv); n < min || n > max {
				return fmt.Sprintf("default %q is %d characters long, outside Len(%d, %d)", *k.dv, n, min, max)
			}
		}
		return ""
	})
	return k
}

// Pattern requires the whole value, when set, to match the regular
// expression re (Go's RE2 syntax), as if it were ^(?:re)$.
func (k *StringKey) Pattern(re string) *StringKey {
	full, err := regexp.Compile(`^(?:` + re + `)$`)
	if err != nil {
		k.problems = append(k.problems, fmt.Sprintf("Pattern %q doesn't compile: %v", re, err))
		return k
	}
	k.checks = append(k.checks, ir.Check{Kind: ir.CheckPattern, Args: []string{re}})
	k.finish = append(k.finish, func() string {
		if k.dv != nil && *k.dv != "" && !full.MatchString(*k.dv) {
			return fmt.Sprintf("default %q doesn't match the pattern %s", *k.dv, re)
		}
		return ""
	})
	return k
}
