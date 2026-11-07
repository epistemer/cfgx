// Package funcref turns a Go function value into a reference that generated
// code can import, or explains why it can't. It is spike S3 (roadmap M1.3).
package funcref

import (
	"errors"
	"fmt"
	"go/token"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

// Ref is an importable, exported, top-level function.
type Ref struct {
	PkgPath string // import path, unescaped
	Name    string // function name
}

func (r Ref) String() string { return r.PkgPath + "." + r.Name }

// Raw returns the symbol name the runtime reports for the function f.
func Raw(f any) (string, error) {
	v := reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		return "", fmt.Errorf("not a function: %T", f)
	}
	if v.IsNil() {
		return "", errors.New("nil function")
	}
	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return "", errors.New("no symbol for function")
	}
	return fn.Name(), nil
}

// Resolve returns an importable reference to f. schemaPkg is the import path
// of the schema package, whose functions generated code must never reference.
func Resolve(f any, schemaPkg string) (Ref, error) {
	raw, err := Raw(f)
	if err != nil {
		return Ref{}, err
	}
	pkg, sym, err := Split(raw)
	if err != nil {
		return Ref{}, err
	}
	switch {
	case strings.Contains(sym, "["):
		return Ref{}, fmt.Errorf("%s: a generic function instance can't be referenced", raw)
	case strings.HasSuffix(sym, "-fm"):
		return Ref{}, fmt.Errorf("%s: a method value can't be referenced", raw)
	case strings.Contains(sym, "."):
		return Ref{}, fmt.Errorf("%s: only top-level functions can be referenced, not closures or methods", raw)
	case !token.IsExported(sym):
		return Ref{}, fmt.Errorf("%s: the function must be exported", raw)
	case pkg == "main":
		return Ref{}, fmt.Errorf("%s: functions in package main can't be imported", raw)
	case pkg == schemaPkg:
		return Ref{}, fmt.Errorf("%s: functions in the schema package can't be referenced from generated code", raw)
	}
	return Ref{PkgPath: pkg, Name: sym}, nil
}

// Split splits a runtime symbol name into its package path and the rest. The
// package path ends at the first dot after the last slash, ignoring anything
// inside generic brackets. The linker escapes dots in the last path element
// as %2e, which Split reverses.
func Split(raw string) (pkgPath, sym string, err error) {
	head := raw
	if i := strings.IndexByte(raw, '['); i >= 0 {
		head = raw[:i]
	}
	start := strings.LastIndexByte(head, '/') + 1
	dot := strings.IndexByte(head[start:], '.')
	if dot < 0 {
		return "", "", fmt.Errorf("%q has no package qualifier", raw)
	}
	dot += start
	pkgPath, err = unescape(raw[:dot])
	if err != nil {
		return "", "", fmt.Errorf("%q: %v", raw, err)
	}
	return pkgPath, raw[dot+1:], nil
}

// unescape reverses the linker's %xx escaping of package paths.
func unescape(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		if i+2 >= len(s) {
			return "", fmt.Errorf("truncated escape in %q", s)
		}
		n, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
		if err != nil {
			return "", fmt.Errorf("bad escape in %q", s)
		}
		b.WriteByte(byte(n))
		i += 2
	}
	return b.String(), nil
}
