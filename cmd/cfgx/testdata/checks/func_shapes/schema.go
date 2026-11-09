package schema

import (
	"strings"

	"github.com/epistemer/cfgx/dsl"
)

type V struct{}

func (V) Check(string) error   { return nil }
func (*V) PCheck(string) error { return nil }

func check(string) error        { return nil }
func Local(string) error        { return nil }
func Gen[T any](T) error        { return nil }
func make1() func(string) error { return func(string) error { return nil } }
func Name() string              { return "x" }
func Contents() []byte          { return nil }
func upper(s string) string     { return strings.ToUpper(s) }

var _ = upper

var G = dsl.Group("g", "G",
	dsl.String("a", "A").Validate(V{}.Check),            // want "g.a: Validate: github.com/epistemer/cfgx/cmd/cfgx/testdata/checks/func_shapes.V.Check-fm is a method"
	dsl.String("c", "C").Validate((&V{}).PCheck),        // want "func_shapes.(*V).PCheck-fm is a method"
	dsl.String("d", "D").Validate(check),                // want "func_shapes.check isn't exported"
	dsl.String("e", "E").Validate(Gen[string]),          // want "is a generic function"
	dsl.String("f", "F").Validate(make1()),              // want "is a closure"
	dsl.String("h", "H").Validate(Local),                // want "g.h: Validate: Local is in the schema package"
	dsl.String("i", "I").Validate(nil),                  // want "g.i: Validate: the function is nil"
	dsl.String("j", "J").DefaultFunc(Name).Default("y"), // want "g.j: has both Default and DefaultFunc"
	dsl.File("k", "K").DefaultFunc(Contents),            // want "g.k: DefaultFunc: a file key's default is a path; use Default"
)

var App = dsl.App("app", "A", G)
