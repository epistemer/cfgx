// Package node parses YAML and JSON configuration files into a format-agnostic
// tree with 1-based source positions.
package node

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

// Kind identifies the kind of a configuration [Node].
type Kind int

const (
	// Null is an explicit null (~, null, an empty value).
	Null Kind = iota
	// Scalar is a string, number, boolean, or timestamp value.
	Scalar
	// Map is an ordered list of key-value pairs.
	Map
	// List is an ordered sequence of child nodes.
	List
)

// Pos is a 1-based line and column.
type Pos struct{ Line, Col int }

// Node is one element in a parsed configuration tree.
type Node struct {
	Kind   Kind
	Scalar string  // the raw scalar text (yaml.Node.Value) for Scalar; "" otherwise
	Tag    string  // the resolved short tag (yaml.Node.ShortTag()): !!str, !!int, !!float, !!bool, !!timestamp, ...
	Quoted bool    // single- or double-quoted, literal or folded
	Map    []Pair  // for Map, in file order
	List   []*Node // for List
	Pos    Pos
}

// Pair is one map entry.
type Pair struct {
	Key    string
	KeyPos Pos
	Value  *Node
}

// Get returns the entry with key, or nil.
func (n *Node) Get(key string) *Pair {
	if n == nil || n.Kind != Map {
		return nil
	}
	for i := range n.Map {
		if n.Map[i].Key == key {
			return &n.Map[i]
		}
	}
	return nil
}

// Format identifies a configuration file format.
type Format int

const (
	// YAML is the YAML 1.2 format (.yaml, .yml).
	YAML Format = iota + 1
	// JSON is the JSON format (.json).
	JSON
)

// String returns the format name ("YAML" or "JSON").
func (f Format) String() string {
	switch f {
	case YAML:
		return "YAML"
	case JSON:
		return "JSON"
	default:
		return ""
	}
}

// FormatOf maps a file name's extension to its format: .yaml and .yml are YAML, .json is JSON. Case-sensitive; anything else is false.
func FormatOf(path string) (Format, bool) {
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		return YAML, true
	case ".json":
		return JSON, true
	default:
		return 0, false
	}
}

// Error is a problem in a config file. Pos is zero when the position is unknown.
type Error struct {
	File string
	Pos  Pos
	Msg  string
}

// Error formats as "file:line:col: msg", or "file: msg" when Pos is zero.
func (e *Error) Error() string {
	if e.Pos == (Pos{}) {
		return fmt.Sprintf("%s: %s", e.File, e.Msg)
	}
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Pos.Line, e.Pos.Col, e.Msg)
}

const quotedStyles = yaml.SingleQuotedStyle | yaml.DoubleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle

// Parse parses data, the contents of file, which is only used in errors. The root is always a Map.
func Parse(file string, data []byte, f Format) (*Node, error) {
	if f == JSON {
		data = rewriteSurrogates(data)
	}

	var doc yaml.Node
	if err := yaml.Load(data, &doc); err != nil {
		if errors.Is(err, io.EOF) {
			return &Node{Kind: Map, Pos: Pos{Line: 1, Col: 1}}, nil
		}
		var loadErr *yaml.LoadError
		if errors.As(err, &loadErr) {
			// yaml.Load wraps Loader.Load's io.EOF into a LoadError with
			// Message "yaml: no documents in stream" without chaining io.EOF.
			if loadErr.Message == "yaml: no documents in stream" {
				return &Node{Kind: Map, Pos: Pos{Line: 1, Col: 1}}, nil
			}
			msg := loadErr.Message
			if msg == "" {
				msg = err.Error()
			}
			return nil, &Error{
				File: file,
				Pos:  Pos{Line: loadErr.Mark.Line, Col: loadErr.Mark.Column},
				Msg:  msg,
			}
		}
		return nil, &Error{
			File: file,
			Msg:  err.Error(),
		}
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return &Node{Kind: Map, Pos: Pos{Line: 1, Col: 1}}, nil
	}

	root := doc.Content[0]
	if root.Anchor != "" || root.Kind == yaml.AliasNode {
		return nil, &Error{
			File: file,
			Pos:  Pos{Line: root.Line, Col: root.Column},
			Msg:  "anchors and aliases aren't supported",
		}
	}
	if root.Kind == yaml.ScalarNode && root.ShortTag() == "!!null" {
		return &Node{Kind: Map, Pos: Pos{Line: 1, Col: 1}}, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, &Error{
			File: file,
			Pos:  Pos{Line: root.Line, Col: root.Column},
			Msg:  "the top level must be a map of settings",
		}
	}

	return walk(file, root)
}

func walk(file string, n *yaml.Node) (*Node, error) {
	pos := Pos{Line: n.Line, Col: n.Column}
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		return nil, &Error{
			File: file,
			Pos:  pos,
			Msg:  "anchors and aliases aren't supported",
		}
	}

	switch n.Kind {
	case yaml.MappingNode:
		var pairs []Pair
		if len(n.Content) > 0 {
			pairs = make([]Pair, 0, len(n.Content)/2)
		}
		seen := make(map[string]Pos, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			kNode := n.Content[i]
			vNode := n.Content[i+1]
			kPos := Pos{Line: kNode.Line, Col: kNode.Column}

			if kNode.Anchor != "" || kNode.Kind == yaml.AliasNode {
				return nil, &Error{
					File: file,
					Pos:  kPos,
					Msg:  "anchors and aliases aren't supported",
				}
			}
			if kNode.ShortTag() == "!!merge" {
				return nil, &Error{
					File: file,
					Pos:  kPos,
					Msg:  "merge keys (<<) aren't supported",
				}
			}
			if kNode.Kind != yaml.ScalarNode {
				return nil, &Error{
					File: file,
					Pos:  kPos,
					Msg:  "map keys must be scalars",
				}
			}
			key := kNode.Value
			if first, dup := seen[key]; dup {
				return nil, &Error{
					File: file,
					Pos:  kPos,
					Msg:  fmt.Sprintf("duplicate key %q (first at %d:%d)", key, first.Line, first.Col),
				}
			}
			seen[key] = kPos

			val, err := walk(file, vNode)
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, Pair{
				Key:    key,
				KeyPos: kPos,
				Value:  val,
			})
		}
		return &Node{
			Kind: Map,
			Map:  pairs,
			Pos:  pos,
		}, nil

	case yaml.SequenceNode:
		var items []*Node
		if len(n.Content) > 0 {
			items = make([]*Node, 0, len(n.Content))
		}
		for _, child := range n.Content {
			elem, err := walk(file, child)
			if err != nil {
				return nil, err
			}
			items = append(items, elem)
		}
		return &Node{
			Kind: List,
			List: items,
			Pos:  pos,
		}, nil

	case yaml.ScalarNode:
		tag := n.ShortTag()
		if tag == "!!null" {
			return &Node{
				Kind: Null,
				Pos:  pos,
			}, nil
		}
		return &Node{
			Kind:   Scalar,
			Scalar: n.Value,
			Tag:    tag,
			Quoted: n.Style&quotedStyles != 0,
			Pos:    pos,
		}, nil

	default:
		return nil, &Error{
			File: file,
			Pos:  pos,
			Msg:  fmt.Sprintf("unexpected YAML node kind %v", n.Kind),
		}
	}
}
