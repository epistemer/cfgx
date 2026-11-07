// Package yaml lives in a directory whose name contains a dot, like
// gopkg.in/yaml.v3, so its symbol names carry an escaped dot.
package yaml

// Dotted is an exported function in that package.
func Dotted(s string) error { return nil }
