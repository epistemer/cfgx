package cfgx

import (
	"fmt"
	"strings"
)

// Problem is one thing wrong with the configuration.
type Problem struct {
	Key     string // the setting, e.g. database.port
	Message string // what is wrong with it
	Source  string // where the value came from, e.g. "env ZP_DATABASE_PORT"; "" if nowhere
}

// Errors is every problem found while loading, sorted by key. Loading
// reports them all at once.
type Errors []Problem

func (e Errors) Error() string {
	var b strings.Builder
	if len(e) == 1 {
		b.WriteString("invalid configuration (1 problem)")
	} else {
		fmt.Fprintf(&b, "invalid configuration (%d problems)", len(e))
	}
	for _, p := range e {
		fmt.Fprintf(&b, "\n  %s: %s", p.Key, p.Message)
		if p.Source != "" {
			fmt.Fprintf(&b, " (%s)", p.Source)
		}
	}
	return b.String()
}
