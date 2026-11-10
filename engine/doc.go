// Package engine is the runtime behind generated code: it loads, merges,
// validates and writes configuration for the settings package cfgx
// generates. Applications and libraries don't import it; they use the
// generated package and package cfgx.
//
// It is exported only so that generated code can import it, and it has no
// compatibility promise: its API changes whenever the generator needs it
// to, in any release. Generated code refers to GeneratedV1, so code from a
// generator that doesn't match this package fails to compile instead of
// misbehaving. Regenerate after upgrading cfgx.
package engine
