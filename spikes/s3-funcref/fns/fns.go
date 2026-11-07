// Package fns holds functions of every shape that a schema might pass to
// Validate, DefaultFunc or Complete.
package fns

import "cmp"

// Email is an exported top-level function: the only shape cfgx accepts.
func Email(s string) error { return nil }

func email(s string) error { return nil }

// Unexported exposes an unexported function as a value.
var Unexported = email

// V has methods, for method values and method expressions.
type V struct{}

// Check is a value-receiver method.
func (V) Check(s string) error { return nil }

// PCheck is a pointer-receiver method.
func (*V) PCheck(s string) error { return nil }

// Max is generic; Max[int] is an instance.
func Max[T cmp.Ordered](v T) error { return nil }

// Make returns a closure.
func Make() func(string) error {
	n := 0
	return func(s string) error { n++; return nil }
}

// PackageClosure is a closure assigned at package level.
var PackageClosure = func(s string) error { return nil }
