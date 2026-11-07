package s4yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	goccyast "github.com/goccy/go-yaml/ast"
	goccyparser "github.com/goccy/go-yaml/parser"
	goccytoken "github.com/goccy/go-yaml/token"
	yamlv3 "go.yaml.in/yaml/v3"
	yamlv4 "go.yaml.in/yaml/v4"
	gopkgv3 "gopkg.in/yaml.v3"
)

// Kind identifies the kind of a parsed configuration node.
type Kind uint8

const (
	KindInvalid Kind = iota
	KindMap
	KindSeq
	KindScalar
	KindNull
)

func (k Kind) String() string {
	switch k {
	case KindMap:
		return "map"
	case KindSeq:
		return "seq"
	case KindScalar:
		return "scalar"
	case KindNull:
		return "null"
	default:
		return "invalid"
	}
}

// Pos records a 1-based line and column in the source file.
type Pos struct {
	Line   int
	Column int
}

// MapEntry stores a key-value pair inside a KindMap node, preserving both key
// and value source positions.
type MapEntry struct {
	Key    string
	KeyPos Pos
	Value  *Node
}

// Node is a prototype of cfgx's internal/node representation.
type Node struct {
	Kind   Kind
	Scalar string
	Tag    string
	Quoted bool
	Map    map[string]MapEntry
	Keys   []string // deterministic insertion order
	List   []*Node
	Pos    Pos
}

// Lookup walks a dot-free path of map keys for test assertions.
func (n *Node) Lookup(keys ...string) (*Node, bool) {
	cur := n
	for _, k := range keys {
		if cur == nil || cur.Kind != KindMap {
			return nil, false
		}
		entry, ok := cur.Map[k]
		if !ok {
			return nil, false
		}
		cur = entry.Value
	}
	return cur, true
}

// LookupEntry returns the MapEntry (including KeyPos) for a path of keys.
func (n *Node) LookupEntry(keys ...string) (MapEntry, bool) {
	if len(keys) == 0 {
		return MapEntry{}, false
	}
	parent, ok := n.Lookup(keys[:len(keys)-1]...)
	if !ok || parent == nil || parent.Kind != KindMap {
		return MapEntry{}, false
	}
	entry, ok := parent.Map[keys[len(keys)-1]]
	return entry, ok
}

// ParseV4 parses YAML or JSON bytes into a Node tree using go.yaml.in/yaml/v4.
// It rejects multi-document streams, duplicate mapping keys, aliases, and merge keys.
func ParseV4(data []byte) (*Node, error) {
	var doc yamlv4.Node
	if err := yamlv4.Load(data, &doc); err != nil {
		return nil, err
	}
	return walkV4Doc(&doc, true)
}

// ParseV4AllowAliases parses YAML or JSON bytes using go.yaml.in/yaml/v4 without
// rejecting aliases, bounded by maxExpansions to prevent billion-laughs bombs.
func ParseV4AllowAliases(data []byte, maxExpansions int) (*Node, error) {
	var doc yamlv4.Node
	if err := yamlv4.Load(data, &doc); err != nil {
		return nil, err
	}
	expansions := 0
	return walkV4NodeBounded(docRootV4(&doc), true, &expansions, maxExpansions)
}

func docRootV4(doc *yamlv4.Node) *yamlv4.Node {
	if doc.Kind == yamlv4.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

func walkV4Doc(doc *yamlv4.Node, strictDupes bool) (*Node, error) {
	if doc.Kind == 0 {
		return &Node{Kind: KindMap, Map: map[string]MapEntry{}}, nil
	}
	if doc.Kind != yamlv4.DocumentNode || len(doc.Content) != 1 {
		return nil, fmt.Errorf("unexpected root node kind %v (content len %d)", doc.Kind, len(doc.Content))
	}
	expansions := 0
	return walkV4NodeBounded(doc.Content[0], strictDupes, &expansions, 0)
}

func walkV4NodeBounded(n *yamlv4.Node, strictDupes bool, expansions *int, maxExpansions int) (*Node, error) {
	pos := Pos{Line: n.Line, Column: n.Column}
	switch n.Kind {
	case yamlv4.MappingNode:
		out := &Node{
			Kind: KindMap,
			Tag:  n.ShortTag(),
			Map:  make(map[string]MapEntry, len(n.Content)/2),
			Keys: make([]string, 0, len(n.Content)/2),
			Pos:  pos,
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			kNode := n.Content[i]
			vNode := n.Content[i+1]
			if kNode.Kind != yamlv4.ScalarNode {
				return nil, fmt.Errorf("%d:%d: non-scalar mapping key (kind=%v, tag=%s)", kNode.Line, kNode.Column, kNode.Kind, kNode.ShortTag())
			}
			if kNode.ShortTag() == "!!merge" && kNode.Style == 0 {
				return nil, fmt.Errorf("%d:%d: YAML merge key '<<' is not supported", kNode.Line, kNode.Column)
			}
			key := kNode.Value
			keyPos := Pos{Line: kNode.Line, Column: kNode.Column}
			if strictDupes {
				if prev, exists := out.Map[key]; exists {
					return nil, fmt.Errorf("%d:%d: duplicate mapping key %q (already defined at %d:%d)",
						keyPos.Line, keyPos.Column, key, prev.KeyPos.Line, prev.KeyPos.Column)
				}
			}
			val, err := walkV4NodeBounded(vNode, strictDupes, expansions, maxExpansions)
			if err != nil {
				return nil, err
			}
			out.Map[key] = MapEntry{Key: key, KeyPos: keyPos, Value: val}
			out.Keys = append(out.Keys, key)
		}
		return out, nil

	case yamlv4.SequenceNode:
		out := &Node{
			Kind: KindSeq,
			Tag:  n.ShortTag(),
			List: make([]*Node, 0, len(n.Content)),
			Pos:  pos,
		}
		for _, elem := range n.Content {
			child, err := walkV4NodeBounded(elem, strictDupes, expansions, maxExpansions)
			if err != nil {
				return nil, err
			}
			out.List = append(out.List, child)
		}
		return out, nil

	case yamlv4.ScalarNode:
		tag := n.ShortTag()
		quoted := n.Style&(yamlv4.SingleQuotedStyle|yamlv4.DoubleQuotedStyle|yamlv4.LiteralStyle|yamlv4.FoldedStyle) != 0
		if tag == "!!null" && !quoted {
			return &Node{
				Kind:   KindNull,
				Scalar: n.Value,
				Tag:    tag,
				Quoted: false,
				Pos:    pos,
			}, nil
		}
		return &Node{
			Kind:   KindScalar,
			Scalar: n.Value,
			Tag:    tag,
			Quoted: quoted,
			Pos:    pos,
		}, nil

	case yamlv4.AliasNode:
		if maxExpansions <= 0 {
			return nil, fmt.Errorf("%d:%d: YAML alias %q is not allowed", n.Line, n.Column, n.Value)
		}
		*expansions++
		if *expansions > maxExpansions {
			return nil, fmt.Errorf("%d:%d: YAML alias expansion limit (%d) exceeded", n.Line, n.Column, maxExpansions)
		}
		if n.Alias == nil {
			return nil, fmt.Errorf("%d:%d: unresolved YAML alias %q", n.Line, n.Column, n.Value)
		}
		return walkV4NodeBounded(n.Alias, strictDupes, expansions, maxExpansions)

	default:
		return nil, fmt.Errorf("%d:%d: unsupported YAML node kind %v", n.Line, n.Column, n.Kind)
	}
}

// ParseV3 parses YAML or JSON bytes into a Node tree using go.yaml.in/yaml/v3.
// Unlike v4's yaml.Load, v3's yaml.Unmarshal silently ignores trailing documents,
// so ParseV3 uses yaml.NewDecoder and checks for a second document.
func ParseV3(data []byte) (*Node, error) {
	dec := yamlv3.NewDecoder(bytes.NewReader(data))
	var doc yamlv3.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	var extra yamlv3.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%d:%d: expected single document, found multiple", extra.Line, extra.Column)
		}
		return nil, err
	}
	if doc.Kind != yamlv3.DocumentNode || len(doc.Content) != 1 {
		return nil, fmt.Errorf("unexpected root node kind %v", doc.Kind)
	}
	return walkV3Node(doc.Content[0])
}

func walkV3Node(n *yamlv3.Node) (*Node, error) {
	pos := Pos{Line: n.Line, Column: n.Column}
	switch n.Kind {
	case yamlv3.MappingNode:
		out := &Node{
			Kind: KindMap,
			Tag:  n.ShortTag(),
			Map:  make(map[string]MapEntry, len(n.Content)/2),
			Keys: make([]string, 0, len(n.Content)/2),
			Pos:  pos,
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			kNode := n.Content[i]
			vNode := n.Content[i+1]
			if kNode.Kind != yamlv3.ScalarNode {
				return nil, fmt.Errorf("%d:%d: non-scalar mapping key", kNode.Line, kNode.Column)
			}
			if kNode.ShortTag() == "!!merge" && kNode.Style == 0 {
				return nil, fmt.Errorf("%d:%d: YAML merge key '<<' is not supported", kNode.Line, kNode.Column)
			}
			key := kNode.Value
			keyPos := Pos{Line: kNode.Line, Column: kNode.Column}
			if prev, exists := out.Map[key]; exists {
				return nil, fmt.Errorf("%d:%d: duplicate mapping key %q (already defined at %d:%d)",
					keyPos.Line, keyPos.Column, key, prev.KeyPos.Line, prev.KeyPos.Column)
			}
			val, err := walkV3Node(vNode)
			if err != nil {
				return nil, err
			}
			out.Map[key] = MapEntry{Key: key, KeyPos: keyPos, Value: val}
			out.Keys = append(out.Keys, key)
		}
		return out, nil

	case yamlv3.SequenceNode:
		out := &Node{
			Kind: KindSeq,
			Tag:  n.ShortTag(),
			List: make([]*Node, 0, len(n.Content)),
			Pos:  pos,
		}
		for _, elem := range n.Content {
			child, err := walkV3Node(elem)
			if err != nil {
				return nil, err
			}
			out.List = append(out.List, child)
		}
		return out, nil

	case yamlv3.ScalarNode:
		tag := n.ShortTag()
		quoted := n.Style&(yamlv3.SingleQuotedStyle|yamlv3.DoubleQuotedStyle|yamlv3.LiteralStyle|yamlv3.FoldedStyle) != 0
		if tag == "!!null" && !quoted {
			return &Node{
				Kind:   KindNull,
				Scalar: n.Value,
				Tag:    tag,
				Quoted: false,
				Pos:    pos,
			}, nil
		}
		return &Node{
			Kind:   KindScalar,
			Scalar: n.Value,
			Tag:    tag,
			Quoted: quoted,
			Pos:    pos,
		}, nil

	case yamlv3.AliasNode:
		return nil, fmt.Errorf("%d:%d: YAML alias %q is not allowed", n.Line, n.Column, n.Value)

	default:
		return nil, fmt.Errorf("%d:%d: unsupported YAML node kind %v", n.Line, n.Column, n.Kind)
	}
}

// ParseGopkgV3 parses YAML or JSON bytes into a Node tree using gopkg.in/yaml.v3 (baseline).
func ParseGopkgV3(data []byte) (*Node, error) {
	dec := gopkgv3.NewDecoder(bytes.NewReader(data))
	var doc gopkgv3.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	var extra gopkgv3.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%d:%d: expected single document, found multiple", extra.Line, extra.Column)
		}
		return nil, err
	}
	if doc.Kind != gopkgv3.DocumentNode || len(doc.Content) != 1 {
		return nil, fmt.Errorf("unexpected root node kind %v", doc.Kind)
	}
	return walkGopkgV3Node(doc.Content[0])
}

func walkGopkgV3Node(n *gopkgv3.Node) (*Node, error) {
	pos := Pos{Line: n.Line, Column: n.Column}
	switch n.Kind {
	case gopkgv3.MappingNode:
		out := &Node{
			Kind: KindMap,
			Tag:  n.ShortTag(),
			Map:  make(map[string]MapEntry, len(n.Content)/2),
			Keys: make([]string, 0, len(n.Content)/2),
			Pos:  pos,
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			kNode := n.Content[i]
			vNode := n.Content[i+1]
			if kNode.Kind != gopkgv3.ScalarNode {
				return nil, fmt.Errorf("%d:%d: non-scalar mapping key", kNode.Line, kNode.Column)
			}
			if kNode.ShortTag() == "!!merge" && kNode.Style == 0 {
				return nil, fmt.Errorf("%d:%d: YAML merge key '<<' is not supported", kNode.Line, kNode.Column)
			}
			key := kNode.Value
			keyPos := Pos{Line: kNode.Line, Column: kNode.Column}
			if prev, exists := out.Map[key]; exists {
				return nil, fmt.Errorf("%d:%d: duplicate mapping key %q (already defined at %d:%d)",
					keyPos.Line, keyPos.Column, key, prev.KeyPos.Line, prev.KeyPos.Column)
			}
			val, err := walkGopkgV3Node(vNode)
			if err != nil {
				return nil, err
			}
			out.Map[key] = MapEntry{Key: key, KeyPos: keyPos, Value: val}
			out.Keys = append(out.Keys, key)
		}
		return out, nil

	case gopkgv3.SequenceNode:
		out := &Node{
			Kind: KindSeq,
			Tag:  n.ShortTag(),
			List: make([]*Node, 0, len(n.Content)),
			Pos:  pos,
		}
		for _, elem := range n.Content {
			child, err := walkGopkgV3Node(elem)
			if err != nil {
				return nil, err
			}
			out.List = append(out.List, child)
		}
		return out, nil

	case gopkgv3.ScalarNode:
		tag := n.ShortTag()
		quoted := n.Style&(gopkgv3.SingleQuotedStyle|gopkgv3.DoubleQuotedStyle|gopkgv3.LiteralStyle|gopkgv3.FoldedStyle) != 0
		if tag == "!!null" && !quoted {
			return &Node{
				Kind:   KindNull,
				Scalar: n.Value,
				Tag:    tag,
				Quoted: false,
				Pos:    pos,
			}, nil
		}
		return &Node{
			Kind:   KindScalar,
			Scalar: n.Value,
			Tag:    tag,
			Quoted: quoted,
			Pos:    pos,
		}, nil

	case gopkgv3.AliasNode:
		return nil, fmt.Errorf("%d:%d: YAML alias %q is not allowed", n.Line, n.Column, n.Value)

	default:
		return nil, fmt.Errorf("%d:%d: unsupported YAML node kind %v", n.Line, n.Column, n.Kind)
	}
}

// ParseGoccy parses YAML or JSON bytes into a Node tree using github.com/goccy/go-yaml.
func ParseGoccy(data []byte) (*Node, error) {
	file, err := goccyparser.ParseBytes(data, 0)
	if err != nil {
		return nil, err
	}
	if len(file.Docs) != 1 || file.Docs[0].Body == nil {
		return nil, fmt.Errorf("expected single document, found %d", len(file.Docs))
	}
	return walkGoccyNode(file.Docs[0].Body)
}

func goccyPos(n goccyast.Node) Pos {
	if n == nil || n.GetToken() == nil || n.GetToken().Position == nil {
		return Pos{}
	}
	p := n.GetToken().Position
	return Pos{Line: p.Line, Column: p.Column}
}

func walkGoccyNode(n goccyast.Node) (*Node, error) {
	pos := goccyPos(n)
	switch v := n.(type) {
	case *goccyast.MappingNode:
		out := &Node{
			Kind: KindMap,
			Tag:  "!!map",
			Map:  make(map[string]MapEntry, len(v.Values)),
			Keys: make([]string, 0, len(v.Values)),
			Pos:  pos,
		}
		for _, mv := range v.Values {
			if err := addGoccyMappingValue(out, mv); err != nil {
				return nil, err
			}
		}
		return out, nil

	case *goccyast.MappingValueNode:
		out := &Node{
			Kind: KindMap,
			Tag:  "!!map",
			Map:  make(map[string]MapEntry, 1),
			Keys: make([]string, 0, 1),
			Pos:  pos,
		}
		if err := addGoccyMappingValue(out, v); err != nil {
			return nil, err
		}
		return out, nil

	case *goccyast.SequenceNode:
		out := &Node{
			Kind: KindSeq,
			Tag:  "!!seq",
			List: make([]*Node, 0, len(v.Values)),
			Pos:  pos,
		}
		for _, elem := range v.Values {
			child, err := walkGoccyNode(elem)
			if err != nil {
				return nil, err
			}
			out.List = append(out.List, child)
		}
		return out, nil

	case *goccyast.StringNode:
		tok := v.GetToken()
		quoted := tok != nil && (tok.Type == goccytoken.DoubleQuoteType || tok.Type == goccytoken.SingleQuoteType)
		return &Node{
			Kind:   KindScalar,
			Scalar: v.Value,
			Tag:    "!!str",
			Quoted: quoted,
			Pos:    pos,
		}, nil

	case *goccyast.LiteralNode:
		return &Node{
			Kind:   KindScalar,
			Scalar: v.Value.Value,
			Tag:    "!!str",
			Quoted: true,
			Pos:    pos,
		}, nil

	case *goccyast.IntegerNode:
		return &Node{
			Kind:   KindScalar,
			Scalar: v.GetToken().Value,
			Tag:    "!!int",
			Quoted: false,
			Pos:    pos,
		}, nil

	case *goccyast.FloatNode:
		return &Node{
			Kind:   KindScalar,
			Scalar: v.GetToken().Value,
			Tag:    "!!float",
			Quoted: false,
			Pos:    pos,
		}, nil

	case *goccyast.InfinityNode:
		return &Node{
			Kind:   KindScalar,
			Scalar: v.GetToken().Value,
			Tag:    "!!float",
			Quoted: false,
			Pos:    pos,
		}, nil

	case *goccyast.NanNode:
		return &Node{
			Kind:   KindScalar,
			Scalar: v.GetToken().Value,
			Tag:    "!!float",
			Quoted: false,
			Pos:    pos,
		}, nil

	case *goccyast.BoolNode:
		return &Node{
			Kind:   KindScalar,
			Scalar: v.GetToken().Value,
			Tag:    "!!bool",
			Quoted: false,
			Pos:    pos,
		}, nil

	case *goccyast.NullNode:
		return &Node{
			Kind:   KindNull,
			Scalar: v.GetToken().Value,
			Tag:    "!!null",
			Quoted: false,
			Pos:    pos,
		}, nil

	case *goccyast.TagNode:
		child, err := walkGoccyNode(v.Value)
		if err != nil {
			return nil, err
		}
		if v.Start != nil {
			child.Tag = v.Start.Value
		}
		return child, nil

	case *goccyast.AnchorNode:
		return walkGoccyNode(v.Value)

	case *goccyast.AliasNode:
		return nil, fmt.Errorf("%d:%d: YAML alias is not allowed", pos.Line, pos.Column)

	default:
		return nil, fmt.Errorf("%d:%d: unsupported goccy AST node %T", pos.Line, pos.Column, n)
	}
}

func addGoccyMappingValue(out *Node, mv *goccyast.MappingValueNode) error {
	if _, isMerge := mv.Key.(*goccyast.MergeKeyNode); isMerge {
		p := goccyPos(mv.Key)
		return fmt.Errorf("%d:%d: YAML merge key '<<' is not supported", p.Line, p.Column)
	}
	keyNode, err := walkGoccyNode(mv.Key)
	if err != nil {
		return err
	}
	if keyNode.Kind != KindScalar {
		return fmt.Errorf("%d:%d: non-scalar mapping key", keyNode.Pos.Line, keyNode.Pos.Column)
	}
	valNode, err := walkGoccyNode(mv.Value)
	if err != nil {
		return err
	}
	key := keyNode.Scalar
	if prev, exists := out.Map[key]; exists {
		return fmt.Errorf("%d:%d: duplicate mapping key %q (already defined at %d:%d)",
			keyNode.Pos.Line, keyNode.Pos.Column, key, prev.KeyPos.Line, prev.KeyPos.Column)
	}
	out.Map[key] = MapEntry{Key: key, KeyPos: keyNode.Pos, Value: valNode}
	out.Keys = append(out.Keys, key)
	return nil
}
