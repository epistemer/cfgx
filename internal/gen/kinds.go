package gen

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/epistemer/cfgx/engine"
	"github.com/epistemer/cfgx/internal/ir"
)

// kindSpec is how the generator handles one kind of key.
type kindSpec struct {
	goType   string   // the getter's type
	flagType string   // the flag's label in --help
	parse    string   // the engine function that parses raw text
	imports  []string // packages goType needs
	quote    bool     // quote the default in --help
	isBool   bool     // a bool flag: --x means --x=true
	isSet    string   // Go expression, with %s for the field: is it non-zero?
	zero     string   // the canonical text of the zero value
	// canon checks a default's text and returns its canonical text, shown in
	// --help, and a Go expression for the generated code.
	canon func(text string) (canonical, lit string, err error)
}

// kinds lists every kind the generator supports.
var kinds = map[ir.Kind]kindSpec{
	ir.KindString: {goType: "string", flagType: "string", parse: "ParseString", quote: true, isSet: `%s != ""`, zero: "",
		canon: func(s string) (string, string, error) { return s, strconv.Quote(s), nil }},
	ir.KindBool: {goType: "bool", flagType: "bool", parse: "ParseBool", isBool: true, isSet: "%s", zero: "false",
		canon: viaParser(engine.ParseBool, strconv.FormatBool, strconv.FormatBool)},
	ir.KindInt: {goType: "int", flagType: "int", parse: "ParseInt", isSet: "%s != 0", zero: "0",
		canon: viaParser(engine.ParseInt, strconv.Itoa, strconv.Itoa)},
	ir.KindInt64: {goType: "int64", flagType: "int64", parse: "ParseInt64", isSet: "%s != 0", zero: "0",
		canon: viaParser(engine.ParseInt64, i64, i64)},
	ir.KindUint: {goType: "uint", flagType: "uint", parse: "ParseUint", isSet: "%s != 0", zero: "0",
		canon: viaParser(engine.ParseUint, u, u)},
	ir.KindFloat: {goType: "float64", flagType: "float", parse: "ParseFloat", isSet: "%s != 0", zero: "0",
		canon: viaParser(engine.ParseFloat, f64, f64)},
	ir.KindDuration: {goType: "time.Duration", flagType: "duration", parse: "ParseDuration", imports: []string{"time"}, isSet: "%s != 0", zero: "0s",
		canon: viaParser(engine.ParseDuration, time.Duration.String, durationLit)},
	ir.KindByteSize: {goType: "int64", flagType: "bytesize", parse: "ParseByteSize", isSet: "%s != 0", zero: "0",
		canon: viaParser(engine.ParseByteSize, engine.FormatByteSize, i64)},
}

func i64(n int64) string   { return strconv.FormatInt(n, 10) }
func u(n uint) string      { return strconv.FormatUint(uint64(n), 10) }
func f64(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// viaParser canonicalizes a default with the engine's own parser, so a
// default is accepted exactly when the same text in a file or env var is.
func viaParser[T any](parse func(string, *T) error, text, lit func(T) string) func(string) (string, string, error) {
	return func(s string) (string, string, error) {
		var v T
		if err := parse(s, &v); err != nil {
			return "", "", err
		}
		return text(v), lit(v), nil
	}
}

// durationLit writes d in the largest unit that divides it: 5 * time.Minute.
func durationLit(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	for _, u := range []struct {
		name string
		d    time.Duration
	}{{"Hour", time.Hour}, {"Minute", time.Minute}, {"Second", time.Second}, {"Millisecond", time.Millisecond}, {"Microsecond", time.Microsecond}} {
		if d%u.d == 0 {
			return fmt.Sprintf("%d * time.%s", d/u.d, u.name)
		}
	}
	return fmt.Sprintf("time.Duration(%d)", int64(d))
}

// kindInfo fills in km's type-dependent fields from its kind. Packages the
// key's type needs are named through imp.
func kindInfo(km *KeyModel, k ir.Key, imp *importer) error {
	if k.Kind == ir.KindEnum {
		return enumInfo(km, k, imp)
	}
	spec, ok := kinds[k.Kind]
	if !ok {
		return fmt.Errorf("key %s has kind %q, which this cfgx doesn't support", km.Path, k.Kind)
	}
	for _, p := range spec.imports {
		imp.name(p, path.Base(p))
	}
	km.GoType, km.FlagType, km.Parse = spec.goType, spec.flagType, spec.parse
	km.Quote, km.Bool = spec.quote, spec.isBool
	km.IsSet = fmt.Sprintf(spec.isSet, "v."+km.Field)
	km.zero = spec.zero
	km.HasDefault = k.Default != nil
	if km.HasDefault {
		text, lit, err := spec.canon(*k.Default)
		if err != nil {
			return fmt.Errorf("default of %s: %v", km.Path, err)
		}
		km.DefaultText, km.DefaultLit = text, lit
	}
	return nil
}

// enumInfo fills in an enum key's fields: its Go type, named through imp,
// and its values, which become two tables in the generated file.
func enumInfo(km *KeyModel, k ir.Key, imp *importer) error {
	t := k.Type
	if t == nil || len(k.Enum) == 0 {
		return fmt.Errorf("enum key %s has no type or values in the IR (a cfgx bug)", km.Path)
	}
	if t.Path != "" && t.Path == imp.schema {
		return fmt.Errorf("enum type %s.%s of %s is declared in the schema package, which the generated code can't import; declare it in another package", t.Pkg, t.Name, km.Path)
	}
	km.GoType = t.Name
	if t.Path != "" {
		km.GoType = imp.name(t.Path, t.Pkg) + "." + t.Name
	}
	km.FlagType, km.Parse = "string", "ParseEnum"
	km.EnumTexts, km.EnumValues = "enumTexts"+km.Field, "enumValues"+km.Field
	km.ParseArgs = km.EnumTexts + ", " + km.EnumValues + ", "
	zeroLit := "0"
	switch t.Kind {
	case "string":
		zeroLit = `""`
	case "bool":
		zeroLit = "false"
	}
	km.IsSet = fmt.Sprintf("v.%s != %s", km.Field, zeroLit)
	for _, v := range k.Enum {
		// The tables hold bare literals, which their element type converts.
		km.Enum = append(km.Enum, EnumModel{Text: v.Text, Lit: v.Lit})
		if v.Zero {
			km.zero = v.Text
			if k.Required {
				return fmt.Errorf("%s is Required, but its value %q is the zero value of %s, which counts as not set; give that value a non-zero underlying value or drop Required", km.Path, v.Text, km.GoType)
			}
		}
	}
	km.HasDefault = k.Default != nil
	if km.HasDefault {
		i := slices.IndexFunc(km.Enum, func(e EnumModel) bool { return e.Text == *k.Default })
		if i < 0 {
			return fmt.Errorf("default %q of %s isn't one of its values", *k.Default, km.Path)
		}
		km.DefaultText, km.DefaultLit = km.Enum[i].Text, km.Enum[i].Lit
		if t.Path != "" {
			km.DefaultLit = km.GoType + "(" + km.DefaultLit + ")"
		}
	}
	return nil
}

// importer names the packages the generated file imports for its keys'
// types, avoiding the names the file already uses.
type importer struct {
	schema string            // the schema package, which can't be imported
	names  map[string]string // path → name in the generated file
	used   map[string]bool   // paths some key needs
	taken  map[string]bool
}

func newImporter(schema string) *importer {
	im := &importer{schema: schema, used: map[string]bool{}, taken: map[string]bool{},
		names: map[string]string{
			"github.com/epistemer/cfgx":        "cfgx",
			"github.com/epistemer/cfgx/engine": "engine",
			"github.com/spf13/cobra":           "cobra",
			"context":                          "context",
			"slices":                           "slices",
			"time":                             "time",
		}}
	for _, n := range im.names {
		im.taken[n] = true
	}
	// Package-level names of the generated file, and the parameters of its
	// functions, which would shadow an import.
	for _, n := range []string{"values", "keys", "root", "app", "v", "raw", "r", "c", "g", "cmd", "run", "args", "ctx", "err"} {
		im.taken[n] = true
	}
	return im
}

// name returns the name the generated file uses for the package at path,
// whose own name is pkg.
func (im *importer) name(path, pkg string) string {
	im.used[path] = true
	if n, ok := im.names[path]; ok {
		return n
	}
	n := pkg
	for i := 2; im.taken[n]; i++ {
		n = pkg + strconv.Itoa(i)
	}
	im.taken[n] = true
	im.names[path] = n
	return n
}

// imports lists the packages keys need that the template doesn't import
// anyway, sorted by path.
func (im *importer) imports(hasCommands bool) []ImportModel {
	var out []ImportModel
	for p := range im.used {
		switch p {
		case "github.com/epistemer/cfgx", "github.com/epistemer/cfgx/engine", "github.com/spf13/cobra":
			continue
		case "context", "slices":
			if hasCommands {
				continue
			}
		}
		out = append(out, ImportModel{Alias: im.names[p], Path: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
