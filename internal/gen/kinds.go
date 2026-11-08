package gen

import (
	"fmt"
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

// kindInfo fills in km's type-dependent fields from its kind.
func kindInfo(km *KeyModel, k ir.Key) error {
	spec, ok := kinds[k.Kind]
	if !ok {
		return fmt.Errorf("key %s has kind %q, which this cfgx doesn't support", km.Path, k.Kind)
	}
	km.Kind = spec
	km.GoType, km.FlagType, km.Parse = spec.goType, spec.flagType, spec.parse
	km.Quote, km.Bool = spec.quote, spec.isBool
	km.IsSet = fmt.Sprintf(spec.isSet, "v."+km.Field)
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
