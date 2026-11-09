package schema

import "github.com/epistemer/cfgx/dsl"

func local(string) []string { return nil }

var G = dsl.Group("g", "G",
	dsl.Path("a", "A").Complete(dsl.Files("certs/*.pem")),  // want "g.a: Files pattern \"certs/*.pem\" must be *.ext"
	dsl.String("b", "B").Complete(dsl.CompleteFunc(local)), // want "g.b: Complete: github.com/epistemer/cfgx/cmd/cfgx/testdata/checks/complete_bad.local isn't exported"
	dsl.Secret("c", "C").Complete(dsl.Dirs()),              // want "g.c has a completion but no flag"
)

var H = dsl.Group("h", "H",
	dsl.Path("a", "A").Complete(dsl.Files("*..pem", "*.tar.gz")), // want "h.a: Files pattern \"*..pem\" must be *.ext"
	dsl.Enum("env", "Env", "dev", "prod").Complete(dsl.Dirs()),   // want "h.env: an enum completes its values"
)

var App = dsl.App("app", "A", G, H)
