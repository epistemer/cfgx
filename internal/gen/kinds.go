package gen

import (
	"fmt"
	"maps"
	"net/url"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
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
	noFlag   bool     // never a flag (Secret)
	fileEnv  bool     // <ENV>_FILE names a file holding the value
	file     bool     // the setting is a path; the value is the file's contents
	maxFile  int64    // the most fileEnv and file read, in bytes
	resolve  bool     // a path: relative paths from the config file resolve against its directory
	show     string   // Go expression, with %s for the field: the value as config print shows it; "" means the field itself
	items    bool     // a list or map: parsed from items with SetItems
	isMap    bool     // a map: items are key=value
	// canon checks a default's text and returns its canonical text, shown in
	// --help, and a Go expression for the generated code. An empty
	// expression means the engine parses the text when loading instead.
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
	ir.KindDuration: {show: "%s.String()", goType: "time.Duration", flagType: "duration", parse: "ParseDuration", imports: []string{"time"}, isSet: "%s != 0", zero: "0s",
		canon: viaParser(engine.ParseDuration, time.Duration.String, durationLit)},
	ir.KindSecret: {show: "engine.ShowSecret(%s)", goType: "cfgx.Secret", parse: "ParseSecret", isSet: "%s.IsSet()", zero: "",
		noFlag: true, fileEnv: true, maxFile: secretMax,
		canon: func(s string) (string, string, error) {
			// The text only decides whether the default counts as set (A4);
			// it is never shown.
			text := ""
			if s != "" {
				text = "******"
			}
			return text, fmt.Sprintf("cfgx.NewSecret(%q)", s), nil
		}},
	ir.KindSecretFile: {show: "engine.ShowSecret(%s)", goType: "cfgx.Secret", flagType: "file", parse: "ParseSecret", quote: true, isSet: "%s.IsSet()", zero: "",
		file: true, maxFile: secretMax,
		canon: func(s string) (string, string, error) { return s, "", nil }},
	ir.KindPath: {goType: "string", flagType: "path", parse: "ParsePath", quote: true, isSet: `%s != ""`, zero: "",
		resolve: true,
		// Parsed when loading, so it is cleaned for the OS the app runs on.
		canon: func(s string) (string, string, error) { return s, "", nil }},
	ir.KindFile: {show: "engine.ShowBytes(%s)", goType: "[]byte", flagType: "file", parse: "ParseBytes", quote: true, isSet: "len(%s) != 0", zero: "",
		file: true, maxFile: fileMax,
		canon: func(s string) (string, string, error) { return s, "", nil }},
	ir.KindURL: {show: "engine.ShowURL(%s)", goType: "*url.URL", flagType: "url", parse: "ParseURL", imports: []string{"net/url"}, quote: true, isSet: "%s != nil", zero: "",
		canon: func(s string) (string, string, error) {
			var u *url.URL
			if err := engine.ParseURL(s, &u); err != nil {
				return "", "", err
			}
			return s, "", nil
		}},
	ir.KindLocation: {show: "engine.ShowLocation(%s)", goType: "*time.Location", flagType: "location", parse: "ParseLocation", imports: []string{"time"}, isSet: "%s != nil", zero: "",
		// Loaded when the app loads the key, so a zone missing there is a
		// problem rather than a panic.
		canon: func(s string) (string, string, error) {
			var loc *time.Location
			if err := engine.ParseLocation(s, &loc); err != nil {
				return "", "", err
			}
			return s, "", nil
		}},
	ir.KindStrings: {goType: "[]string", flagType: "strings", parse: "ParseStrings", isSet: "len(%s) != 0", zero: "", items: true,
		canon: listCanon(engine.ParseStrings, itemText, strconv.Quote, "[]string")},
	ir.KindInts: {goType: "[]int", flagType: "ints", parse: "ParseInts", isSet: "len(%s) != 0", zero: "", items: true,
		canon: listCanon(engine.ParseInts, strconv.Itoa, strconv.Itoa, "[]int")},
	ir.KindDurations: {show: "engine.ShowDurations(%s)", goType: "[]time.Duration", flagType: "durations", parse: "ParseDurations", imports: []string{"time"}, isSet: "len(%s) != 0", zero: "", items: true,
		canon: listCanon(engine.ParseDurations, time.Duration.String, durationLit, "[]time.Duration")},
	ir.KindStringMap: {goType: "map[string]string", flagType: "key=value", parse: "ParseStringMap", isSet: "len(%s) != 0", zero: "", items: true, isMap: true,
		canon: func(s string) (string, string, error) {
			var m map[string]string
			items, err := engine.SplitCSV(s)
			if err == nil {
				err = engine.ParseStringMap(items, &m)
			}
			if err != nil {
				return "", "", err
			}
			if len(m) == 0 {
				return "", "nil", nil
			}
			keys := slices.Sorted(maps.Keys(m))
			text, lit := make([]string, len(keys)), make([]string, len(keys))
			for i, k := range keys {
				text[i] = itemText(k + "=" + m[k])
				lit[i] = strconv.Quote(k) + ": " + strconv.Quote(m[k])
			}
			return "[" + strings.Join(text, ",") + "]", "map[string]string{" + strings.Join(lit, ", ") + "}", nil
		}},
	ir.KindByteSize: {show: "engine.FormatByteSize(%s)", goType: "int64", flagType: "bytesize", parse: "ParseByteSize", isSet: "%s != 0", zero: "0",
		canon: viaParser(engine.ParseByteSize, engine.FormatByteSize, i64)},
}

// secretMax and fileMax are the most a secret's file and a File key's
// file may hold.
const (
	secretMax = 1 << 20
	fileMax   = 16 << 20
)

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

// listCanon canonicalizes a list default, recorded as one CSV record, with
// the engine's own parser. --help shows [a,b]; an empty list shows nothing
// and is nil.
func listCanon[T any](parse func([]string, *[]T) error, text, lit func(T) string, typ string) func(string) (string, string, error) {
	return func(s string) (string, string, error) {
		var vs []T
		items, err := engine.SplitCSV(s)
		if err == nil {
			err = parse(items, &vs)
		}
		if err != nil {
			return "", "", err
		}
		if len(vs) == 0 {
			return "", "nil", nil
		}
		texts, lits := make([]string, len(vs)), make([]string, len(vs))
		for i, v := range vs {
			texts[i], lits[i] = text(v), lit(v)
		}
		return "[" + strings.Join(texts, ",") + "]", typ + "{" + strings.Join(lits, ", ") + "}", nil
	}
}

// itemText shows a list item in --help: quoted, as in CSV, when it is
// empty or holds a comma, a quote or outer space, so [a,"b,c"] reads right.
func itemText(s string) string {
	if s == "" || strings.ContainsAny(s, ",\"\r\n") || strings.TrimSpace(s) != s {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
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
	switch k.Kind {
	case ir.KindEnum:
		return enumInfo(km, k, imp)
	case ir.KindText:
		return textInfo(km, k, imp)
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
	km.FileEnv, km.File, km.MaxFileSize, km.Resolve = spec.fileEnv, spec.file, spec.maxFile, spec.resolve
	km.Items, km.Map = spec.items, spec.isMap
	km.Show = "v." + km.Field
	if spec.show != "" {
		km.Show = fmt.Sprintf(spec.show, "v."+km.Field)
	}
	if spec.noFlag {
		km.Flag = ""
	}
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
	km.Show = fmt.Sprintf("engine.ShowEnum(v.%s, %s, %s)", km.Field, km.EnumTexts, km.EnumValues)
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

// textInfo fills in a Text key's fields: its Go type, named through imp.
// The default's text was checked by the schema, which parsed it back with
// the type's own UnmarshalText; the engine parses it when loading.
func textInfo(km *KeyModel, k ir.Key, imp *importer) error {
	t := k.Type
	if t == nil || t.Path == "" {
		return fmt.Errorf("text key %s has no type in the IR (a cfgx bug)", km.Path)
	}
	if t.Path == imp.schema {
		return fmt.Errorf("text type %s.%s of %s is declared in the schema package, which the generated code can't import; declare it in another package", t.Pkg, t.Name, km.Path)
	}
	km.GoType = imp.name(t.Path, t.Pkg) + "." + t.Name
	km.FlagType, km.Parse, km.Quote = strings.ToLower(t.Name), "ParseText", true
	km.IsSet = fmt.Sprintf("engine.NonZero(v.%s)", km.Field)
	km.Show = fmt.Sprintf("engine.ShowText(v.%s)", km.Field)
	km.HasDefault = k.Default != nil
	if km.HasDefault {
		km.DefaultText = *k.Default
	}
	return nil
}

// importer names the packages the generated file imports for its keys'
// types, avoiding the names the file already uses.
type importer struct {
	schema string            // the schema package, which can't be imported
	names  map[string]string // path → name in the generated file
	pkgs   map[string]string // path → the package's own name
	used   map[string]bool   // paths some key needs
	taken  map[string]bool
}

func newImporter(schema string) *importer {
	im := &importer{schema: schema, used: map[string]bool{}, pkgs: map[string]string{}, taken: map[string]bool{},
		names: map[string]string{
			"github.com/epistemer/cfgx":        "cfgx",
			"github.com/epistemer/cfgx/engine": "engine",
			"github.com/spf13/cobra":           "cobra",
			"context":                          "context",
			"slices":                           "slices",
			"time":                             "time",
			"net/url":                          "url",
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
	if pkg != "" {
		im.pkgs[path] = pkg
	}
	if n, ok := im.names[path]; ok {
		return n
	}
	base := pkg
	if base == "" {
		base = guessAlias(path)
	}
	n := base
	for i := 2; im.taken[n]; i++ {
		n = base + strconv.Itoa(i)
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
		out = append(out, ImportModel{Alias: im.names[p], Path: p, Pkg: im.pkgs[p]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
