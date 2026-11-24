package dsl

import (
	"bytes"
	"reflect"
	"slices"
	"testing"

	"github.com/epistemer/cfgx/internal/ir"
)

type strStringer string

func (s strStringer) String() string { return "S:" + string(s) }

type intStringer int

func (i intStringer) String() string { return []string{"zero", "one"}[i] }

type ptrMarshaler float32

func (p *ptrMarshaler) MarshalText() ([]byte, error) { return []byte("half"), nil }

func (p ptrMarshaler) String() string { return "ignored" }

// The text is MarshalText, else a string type's own value, else String,
// else the Go literal.
func TestEnumText(t *testing.T) {
	cases := []struct {
		k    *keyBase
		typ  ir.Type
		want []ir.EnumValue
	}{
		{&Enum("a", "A", strStringer("x")).keyBase,
			ir.Type{Path: "github.com/epistemer/cfgx/dsl", Pkg: "dsl", Name: "strStringer", Kind: "string"},
			[]ir.EnumValue{{Text: "x", Lit: `"x"`}}},
		{&Enum("a", "A", intStringer(0), intStringer(1)).keyBase,
			ir.Type{Path: "github.com/epistemer/cfgx/dsl", Pkg: "dsl", Name: "intStringer", Kind: "int"},
			[]ir.EnumValue{{Text: "zero", Lit: "0", Zero: true}, {Text: "one", Lit: "1"}}},
		{&Enum("a", "A", ptrMarshaler(0.5)).keyBase,
			ir.Type{Path: "github.com/epistemer/cfgx/dsl", Pkg: "dsl", Name: "ptrMarshaler", Kind: "float32"},
			[]ir.EnumValue{{Text: "half", Lit: "0.5"}}},
		{&Enum("a", "A", uint8(7), 9).keyBase,
			ir.Type{Name: "uint8", Kind: "uint8"},
			[]ir.EnumValue{{Text: "7", Lit: "7"}, {Text: "9", Lit: "9"}}},
		{&Enum("a", "A", true).keyBase,
			ir.Type{Name: "bool", Kind: "bool"},
			[]ir.EnumValue{{Text: "true", Lit: "true"}}},
	}
	for i, c := range cases {
		if *c.k.typ != c.typ {
			t.Errorf("%d: type = %+v, want %+v", i, *c.k.typ, c.typ)
		}
		if !reflect.DeepEqual(c.k.enum, c.want) {
			t.Errorf("%d: values = %+v, want %+v", i, c.k.enum, c.want)
		}
	}
}

func TestEnumDefault(t *testing.T) {
	k := Enum("a", "A", "x", "y").Default("y")
	if k.def == nil || *k.def != "y" || len(k.problems) != 0 {
		t.Errorf("def = %v, problems = %v", k.def, k.problems)
	}
	k = Enum("a", "A", "x").Default("z")
	if len(k.problems) != 1 {
		t.Errorf("problems = %v", k.problems)
	}
}

// An example, like a default, must be one of the values (review 0037).
func TestEnumExample(t *testing.T) {
	k := Enum("a", "A", "x", "y").Example("y")
	if k.example == nil || *k.example != "y" || len(k.problems) != 0 {
		t.Errorf("example = %v, problems = %v", k.example, k.problems)
	}
	k = Enum("a", "A", "x").Example("z")
	if len(k.problems) != 1 || k.problems[0] != `example "z" isn't one of the values` {
		t.Errorf("problems = %q", k.problems)
	}
}

// A map's default and example keys must survive their text form.
func TestStringMapKeys(t *testing.T) {
	k := StringMap("m", "M").Default(map[string]string{"a": "1"}).Example(map[string]string{"b": "2"})
	if len(k.problems) != 0 || *k.def != "a=1" || *k.example != "b=2" {
		t.Errorf("problems %q, def %v, example %v", k.problems, k.def, k.example)
	}
	k = StringMap("m", "M").Default(map[string]string{"a=b": "1"}).Example(map[string]string{" c": "2"})
	want := []string{`default key "a=b" must be non-empty, without = or outer spaces`, `example key " c" must be non-empty, without = or outer spaces`}
	if !slices.Equal(k.problems, want) {
		t.Errorf("problems = %q", k.problems)
	}
}

type (
	valLabelStr    string
	valLabelCustom int
	ptrLabel       string
	paramLabel     string
	twoResultLabel string
	zeroResult     string
	otherMethod    string
	labelTag       struct{}
)

func (valLabelStr) Label() string           { return "x" }
func (valLabelCustom) Label() labelTag      { return labelTag{} }
func (*ptrLabel) Label() string             { return "x" }
func (paramLabel) Label(x int) string       { return "x" }
func (twoResultLabel) Label() (string, int) { return "x", 0 }
func (zeroResult) Label()                   {}
func (otherMethod) Title() string           { return "x" }

func TestEnumLabeled(t *testing.T) {
	cases := []struct {
		name string
		key  func() KeyDef
		want bool
	}{
		{"value receiver string", func() KeyDef { return Enum("k", "K", valLabelStr("a")) }, true},
		{"value receiver custom result", func() KeyDef { return Enum("k", "K", valLabelCustom(1)) }, true},
		{"plain string", func() KeyDef { return Enum("k", "K", "a", "b") }, false},
		{"pointer receiver", func() KeyDef { return Enum("k", "K", ptrLabel("a")) }, false},
		{"label with parameter", func() KeyDef { return Enum("k", "K", paramLabel("a")) }, false},
		{"label with two results", func() KeyDef { return Enum("k", "K", twoResultLabel("a")) }, false},
		{"label with no results", func() KeyDef { return Enum("k", "K", zeroResult("a")) }, false},
		{"different method name", func() KeyDef { return Enum("k", "K", otherMethod("a")) }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reset()
			g := Group("g", "G", c.key())
			App("app", "App", g)
			var buf bytes.Buffer
			if err := Export(&buf); err != nil {
				t.Fatal(err)
			}
			a, err := ir.Decode(&buf)
			if err != nil {
				t.Fatal(err)
			}
			got := a.Groups[0].Keys[0].EnumLabeled
			if got != c.want {
				t.Errorf("EnumLabeled = %v, want %v", got, c.want)
			}
		})
	}
}
