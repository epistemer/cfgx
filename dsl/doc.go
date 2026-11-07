// Package dsl is the vocabulary for writing a cfgx schema: App, Group, Command,
// Tag, the key constructors and their modifiers.
//
// Only schema packages import it. It runs at generate time and is never linked
// into an application binary.
package dsl
