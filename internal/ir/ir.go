package ir

import (
	"encoding/json"
	"fmt"
	"io"
)

// Version is the IR format version. dsl.Export writes it, and the generator
// refuses IR of any other version, which catches a schema built against a
// different cfgx release than the generator.
const Version = 2

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

// The kinds the walking skeleton (M2) supports.
const (
	KindString Kind = "string"
	KindInt    Kind = "int"
	KindBool   Kind = "bool"
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
	GlobalOrphans []Ref     `json:"globalOrphans,omitempty"`
	Groups        []Group   `json:"groups"`
	Commands      []Command `json:"commands"`
	Pos           Pos       `json:"pos"`
}

// Group is a named set of keys.
type Group struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	Keys []Key  `json:"keys"`
	Pos  Pos    `json:"pos"`
}

// Key is one setting.
type Key struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	Kind Kind   `json:"kind"`
	// Default is the default in canonical text form; nil means none.
	Default  *string `json:"default,omitempty"`
	Required bool    `json:"required,omitempty"`
	Pos      Pos     `json:"pos"`
}

// Command is a config profile: the groups one command uses.
type Command struct {
	Name string   `json:"name"`
	Uses []string `json:"uses"`
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
