// Command cfgx generates typed configuration code and files from a cfgx schema.
//
// Its subcommands (gen, gen --check, diff, init) are planned in docs/roadmap.md;
// none is implemented yet.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "cfgx: not implemented yet; see docs/roadmap.md")
	os.Exit(2)
}
