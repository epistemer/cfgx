package ir

import (
	"encoding/json"
	"fmt"
	"io"
)

// Version is the IR format version. dsl.Export writes it, and the generator
// refuses IR of any other version, which catches a schema built against a
// different cfgx release than the generator.
const Version = 15

// Pos is a position in the schema's source.
type Pos struct {
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

func (p Pos) String() string {
	if p.File == "" {
		return "<unknown position>"
	}
	return fmt.Sprintf("%s:%d", p.File, p.Line)
}

// Kind is the value kind of a key.
type Kind string

// The kinds of keys.
const (
	KindString Kind = "string"
	KindInt    Kind = "int"
	KindBool   Kind = "bool"
	// M4.1
	KindInt64    Kind = "int64"
	KindUint     Kind = "uint"
	KindFloat    Kind = "float"
	KindDuration Kind = "duration"
	KindByteSize Kind = "bytesize"
	// M4.2
	KindEnum Kind = "enum"
	// M4.4
	KindSecret     Kind = "secret"
	KindSecretFile Kind = "secretfile"
	// M4.5
	KindPath     Kind = "path"
	KindFile     Kind = "file"
	KindURL      Kind = "url"
	KindLocation Kind = "location"
	// M4.6
	KindStrings   Kind = "strings"
	KindInts      Kind = "ints"
	KindDurations Kind = "durations"
	KindStringMap Kind = "stringmap"
	// M4.7
	KindText Kind = "text"
)

// App is a whole schema.
type App struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	Desc      string `json:"desc"`
	EnvPrefix string `json:"envPrefix,omitempty"`
	// ConfigFile is the config file's basename; "" means no config file.
	ConfigFile  string   `json:"configFile,omitempty"`
	SearchPaths []string `json:"searchPaths,omitempty"`
	// Formats are the accepted file formats ("yaml", "json"); empty means both.
	Formats []string `json:"formats,omitempty"`
	// Global names the groups every command gets.
	Global []string `json:"global,omitempty"`
	// GlobalOrphans are Global groups that weren't passed to dsl.App.
	GlobalOrphans []Ref `json:"globalOrphans,omitempty"`
	// Initialisms are extra words written in capitals in Go names.
	Initialisms []string  `json:"initialisms,omitempty"`
	Groups      []Group   `json:"groups"`
	Tags        []Tag     `json:"tags,omitempty"`
	Rules       []Rule    `json:"rules,omitempty"`
	Commands    []Command `json:"commands"`
	Pos         Pos       `json:"pos"`
	// Schema is the schema package's import path. The loader sets it; it
	// isn't part of the encoded IR.
	Schema string `json:"-"`
}

// Group is a named set of keys and nested groups.
type Group struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	// GoName replaces the group's segment of Go names; "" derives it.
	GoName string `json:"goName,omitempty"`
	// FlagPrefix replaces the prefix of the group's flags; nil derives it.
	FlagPrefix *string `json:"flagPrefix,omitempty"`
	// Computed names hand-written methods that join the group's interface.
	Computed []string `json:"computed,omitempty"`
	// Tags apply to every key in the group, nested groups included.
	Tags   []string `json:"tags,omitempty"`
	Keys   []Key    `json:"keys"`
	Groups []Group  `json:"groups,omitempty"`
	Pos    Pos      `json:"pos"`
}

// Key is one setting.
type Key struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	Kind Kind   `json:"kind"`
	// Default is the default in canonical text form; nil means none.
	Default    *string  `json:"default,omitempty"`
	Required   bool     `json:"required,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	Deprecated string   `json:"deprecated,omitempty"`
	// Type is the Go type of an enum's values, or of a Text key.
	Type *Type `json:"type,omitempty"`
	// Enum lists an enum's values, in declaration order.
	Enum []EnumValue `json:"enum,omitempty"`
	// Naming and source modifiers (M4.9). Short is a one-letter flag; Flag
	// and Env replace the derived names whole; FileKey replaces the key's
	// segment of its file path, and GoName its segment of Go names. "" keeps
	// the derived name.
	Short   string `json:"short,omitempty"`
	Flag    string `json:"flag,omitempty"`
	Env     string `json:"env,omitempty"`
	FileKey string `json:"fileKey,omitempty"`
	GoName  string `json:"goName,omitempty"`
	// Sources limits where the value may come from: "flag", "env", "file".
	// nil means all three.
	Sources []string `json:"sources,omitempty"`
	Hidden  bool     `json:"hidden,omitempty"`
	// Example is shown in example files instead of the default, as text.
	Example *string `json:"example,omitempty"`
	// Checks are the key's constraints, in declaration order.
	Checks []Check `json:"checks,omitempty"`
	// Validate and DefaultFunc are functions the generated code calls.
	Validate    *FuncRef `json:"validate,omitempty"`
	DefaultFunc *FuncRef `json:"defaultFunc,omitempty"`
	// Problems were found while the schema ran, such as a MarshalText
	// error; the generator reports them at Pos.
	Problems []string `json:"problems,omitempty"`
	Pos      Pos      `json:"pos"`
}

// The kinds of checks.
const (
	CheckMin     = "min"     // Args: the minimum, in the key's text form
	CheckMax     = "max"     // Args: the maximum
	CheckLen     = "len"     // Args: the least and most characters
	CheckPattern = "pattern" // Args: a regular expression the whole value matches
)

// Check is a constraint on a key's value.
type Check struct {
	Kind string   `json:"kind"`
	Args []string `json:"args"`
}

// FuncRef names a top-level function the schema passed, from
// runtime.FuncForPC.
type FuncRef struct {
	// Path is the function's package path, unescaped; Name its name.
	Path string `json:"path"`
	Name string `json:"name"`
	// Problem is why the function can't be referenced, found by the DSL:
	// a closure, a method, an unexported or generic function.
	Problem string `json:"problem,omitempty"`
	Pos     Pos    `json:"pos"`
}

// Type is a Go type the generated code names.
type Type struct {
	// Path is the import path; "" for a predeclared type such as string.
	Path string `json:"path,omitempty"`
	// Pkg is the package's name; Name is the type's name within it.
	Pkg  string `json:"pkg,omitempty"`
	Name string `json:"name"`
	// Kind is the underlying type's reflect kind: string, int, uint8,
	// float64, bool...
	Kind string `json:"kind"`
}

// EnumValue is one value of an enum.
type EnumValue struct {
	// Text is the value's canonical text: what files, env vars and flags
	// say.
	Text string `json:"text"`
	// Lit is a Go literal of the underlying value: "a", 2, 1.5 or true.
	Lit string `json:"lit"`
	// Zero is true for the type's zero value, which counts as not set (A4).
	Zero bool `json:"zero,omitempty"`
}

// RuleKind is a kind of cross-key rule.
type RuleKind string

// The rules (interface §3.5).
const (
	RuleAtMostOne  RuleKind = "AtMostOne"
	RuleExactlyOne RuleKind = "ExactlyOne"
	RuleAllOrNone  RuleKind = "AllOrNone"
	RuleRequires   RuleKind = "Requires"
	RuleRelevantIf RuleKind = "RelevantIf"
)

// Rule is a cross-key constraint declared on a group.
type Rule struct {
	Kind RuleKind `json:"kind"`
	// Group is the path of the group that declared it; problems are
	// reported under it.
	Group string `json:"group"`
	// Keys are the paths of the keys, in declaration order. Requires and
	// RelevantIf have two: a, then b.
	Keys []string `json:"keys"`
	Pos  Pos      `json:"pos"`
}

// Tag is a cross-cutting set of keys with its own interface.
type Tag struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	Pos  Pos    `json:"pos"`
}

// Command is a config profile: the groups one command uses.
type Command struct {
	Name string `json:"name"`
	// Uses are the dotted paths of the groups the command uses.
	Uses []string `json:"uses"`
	// Tags are the tags the command uses.
	Tags []string `json:"tags,omitempty"`
	// Args are the flag-only arguments of the command.
	Args []Key `json:"args,omitempty"`
	// Orphans are groups the command uses that weren't passed to dsl.App.
	// The generator reports them along with every other problem.
	Orphans []Ref `json:"orphans,omitempty"`
	Pos     Pos   `json:"pos"`
}

// Ref names a declaration and where it is.
type Ref struct {
	Name string `json:"name"`
	Pos  Pos    `json:"pos"`
}

// Encode writes a as indented JSON.
func Encode(w io.Writer, a *App) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(a)
}

// Decode reads IR written by Encode and checks its version.
func Decode(r io.Reader) (*App, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var a App
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("decoding the schema's IR: %w", err)
	}
	if a.Version != Version {
		return nil, fmt.Errorf("the schema's IR has version %d, but this cfgx expects %d: cfgx/dsl and cmd/cfgx come from different cfgx versions", a.Version, Version)
	}
	return &a, nil
}
