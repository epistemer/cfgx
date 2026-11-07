// Package store is the built-in bbolt store for runtime-mutable settings, and
// the admin API that a settings dashboard uses (docs/interface.md §7.3).
//
// Generated code imports it only when a schema declares a store, so programs
// without mutable settings don't link bbolt.
package store
