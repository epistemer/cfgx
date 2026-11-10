package dsl

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// Lossy writes its value but always parses back to 1.
type Lossy int

func (l Lossy) MarshalText() ([]byte, error) { return []byte("x"), nil }
func (l *Lossy) UnmarshalText([]byte) error  { *l = 1; return nil }

// Picky writes text it can't parse.
type Picky int

func (p Picky) MarshalText() ([]byte, error) { return []byte("p"), nil }
func (p *Picky) UnmarshalText([]byte) error  { return errors.New("nope") }

// Silent writes empty text for every value.
type Silent int

func (s Silent) MarshalText() ([]byte, error) { return nil, nil }
func (s *Silent) UnmarshalText([]byte) error  { return nil }

func TestTextDefault(t *testing.T) {
	k := Text("cidr", "Range", netip.Prefix{}).Default(netip.MustParsePrefix("10.0.0.0/8"))
	if len(k.problems) != 0 || *k.def != "10.0.0.0/8" || k.typ.Path != "net/netip" || k.typ.Pkg != "netip" {
		t.Errorf("netip.Prefix: problems %q, def %q, type %+v", k.problems, *k.def, k.typ)
	}
	for _, c := range []struct {
		k    *keyBase
		want string
	}{
		{&Text("l", "L", Lossy(0)).Default(Lossy(2)).keyBase, `default "x" parses back to a different value`},
		{&Text("p", "P", Picky(0)).Default(Picky(2)).keyBase, `default "p" doesn't parse back: nope`},
		{&Text("s", "S", Silent(0)).Default(Silent(3)).keyBase, "default writes empty text, which means not set"},
	} {
		if len(c.k.problems) != 1 || !strings.Contains(c.k.problems[0], c.want) {
			t.Errorf("%s: problems %q; want %q", c.k.name, c.k.problems, c.want)
		}
	}
}

// ReadOnly parses text but can't write it.
type ReadOnly int

func (r *ReadOnly) UnmarshalText([]byte) error { return nil }

// A Mutable Text key needs MarshalText: a write stores the value as text.
func TestTextMutable(t *testing.T) {
	ir := Text("r", "R", ReadOnly(0)).Mutable().export(nil)
	if len(ir.Problems) != 1 || !strings.Contains(ir.Problems[0], "is Mutable, which needs dsl.ReadOnly to implement encoding.TextMarshaler") {
		t.Errorf("problems %q", ir.Problems)
	}
	if ir := Text("c", "C", netip.Prefix{}).Mutable().export(nil); len(ir.Problems) != 0 {
		t.Errorf("netip.Prefix: problems %q", ir.Problems)
	}
}
