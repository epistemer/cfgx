// Package binding is spike S2 (roadmap M1.2): the flag binding cfgx's engine
// will use. Every key gets a pflag.Value that only records raw text; defaults
// are shown in --help but never applied by pflag, so a flag counts only when
// it was actually passed.
package binding

import (
	"encoding/csv"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// rawValue records the text of a scalar flag.
type rawValue struct {
	typ string
	raw string
}

func (v *rawValue) String() string     { return v.raw }
func (v *rawValue) Set(s string) error { v.raw = s; return nil }
func (v *rawValue) Type() string       { return v.typ }

// sliceValue accumulates every occurrence of a repeatable flag, splitting
// comma-separated values the way pflag's own slices do.
type sliceValue struct {
	typ   string
	items []string
}

func (v *sliceValue) String() string { return "[" + strings.Join(v.items, ",") + "]" }
func (v *sliceValue) Type() string   { return v.typ }

func (v *sliceValue) Set(s string) error {
	parts, err := splitCSV(s)
	if err != nil {
		return err
	}
	v.items = append(v.items, parts...)
	return nil
}

// Append, Replace and GetSlice implement pflag.SliceValue.
func (v *sliceValue) Append(s string) error     { v.items = append(v.items, s); return nil }
func (v *sliceValue) Replace(ss []string) error { v.items = append([]string(nil), ss...); return nil }
func (v *sliceValue) GetSlice() []string        { return append([]string(nil), v.items...) }

func splitCSV(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	return csv.NewReader(strings.NewReader(s)).Read()
}

// Key describes how one setting is bound, as the generated tables will.
type Key struct {
	Flag, Short, Env string
	Type             string // pflag type name shown in help: "int", "string", "bool", "duration", "stringSlice"
	Default          string // display text; never applied by pflag
	Usage            string
	Slice            bool
}

// Bind registers k on fs.
func Bind(fs *pflag.FlagSet, k Key) *pflag.Flag {
	var v pflag.Value = &rawValue{typ: k.Type}
	if k.Slice {
		v = &sliceValue{typ: k.Type}
	}
	f := fs.VarPF(v, k.Flag, k.Short, k.Usage)
	f.DefValue = k.Default
	if k.Type == "bool" {
		f.NoOptDefVal = "true"
	}
	return f
}

// Resolve applies the precedence flag > env > default. A flag counts only if
// it was passed (Changed); its default never hides an env var.
func Resolve(fs *pflag.FlagSet, k Key) (raw []string, source string) {
	if f := fs.Lookup(k.Flag); f != nil && f.Changed {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			return sv.GetSlice(), "flag --" + k.Flag
		}
		return []string{f.Value.String()}, "flag --" + k.Flag
	}
	if v, ok := os.LookupEnv(k.Env); ok {
		if k.Slice {
			parts, _ := splitCSV(v)
			return parts, "env " + k.Env
		}
		return []string{v}, "env " + k.Env
	}
	if k.Slice {
		parts, _ := splitCSV(k.Default)
		return parts, "default"
	}
	return []string{k.Default}, "default"
}
