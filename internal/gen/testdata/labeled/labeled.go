// Package labeled declares a named enum type with a Label() method for tests.
package labeled

// Mode is a named string enum that labels its own values.
type Mode string

const (
	Sequential Mode = "sequential"
	Parallel   Mode = "parallel"
)

// Label returns the value's label.
func (m Mode) Label() string { return "mode." + string(m) }
