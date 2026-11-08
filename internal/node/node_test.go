package node

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestFormatOf(t *testing.T) {
	tests := []struct {
		path   string
		wantF  Format
		wantOK bool
	}{
		{path: "config.yaml", wantF: YAML, wantOK: true},
		{path: "dir/sub/config.yml", wantF: YAML, wantOK: true},
		{path: "/etc/app/config.json", wantF: JSON, wantOK: true},
		{path: ".yaml", wantF: YAML, wantOK: true},
		{path: ".yml", wantF: YAML, wantOK: true},
		{path: ".json", wantF: JSON, wantOK: true},
		{path: "config.YAML", wantF: 0, wantOK: false},
		{path: "config.YML", wantF: 0, wantOK: false},
		{path: "config.JSON", wantF: 0, wantOK: false},
		{path: "config.toml", wantF: 0, wantOK: false},
		{path: "config.yaml.bak", wantF: 0, wantOK: false},
		{path: "config", wantF: 0, wantOK: false},
		{path: "", wantF: 0, wantOK: false},
	}

	for _, tc := range tests {
		gotF, gotOK := FormatOf(tc.path)
		if gotF != tc.wantF || gotOK != tc.wantOK {
			t.Errorf("FormatOf(%q) = (%v, %v), want (%v, %v)", tc.path, gotF, gotOK, tc.wantF, tc.wantOK)
		}
	}

	if got := YAML.String(); got != "YAML" {
		t.Errorf("YAML.String() = %q, want %q", got, "YAML")
	}
	if got := JSON.String(); got != "JSON" {
		t.Errorf("JSON.String() = %q, want %q", got, "JSON")
	}
	if got := Format(0).String(); got != "" {
		t.Errorf("Format(0).String() = %q, want %q", got, "")
	}
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name string
		err  Error
		want string
	}{
		{
			name: "with_pos",
			err:  Error{File: "config.yaml", Pos: Pos{Line: 12, Col: 5}, Msg: "invalid value"},
			want: "config.yaml:12:5: invalid value",
		},
		{
			name: "zero_pos",
			err:  Error{File: "config.yaml", Pos: Pos{}, Msg: "expected single document"},
			want: "config.yaml: expected single document",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error.Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseNestedYAMLPositionsAndOrder(t *testing.T) {
	const src = `server:
  host: "localhost"
  port: 8080
  tls:
    enabled: true
    cert: ~
  origins:
    - https://a.example.com
    - "https://b.example.com"
    -
database:
  name: app
`
	root, err := Parse("app.yaml", []byte(src), YAML)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	want := &Node{
		Kind: Map,
		Pos:  Pos{Line: 1, Col: 1},
		Map: []Pair{
			{
				Key:    "server",
				KeyPos: Pos{Line: 1, Col: 1},
				Value: &Node{
					Kind: Map,
					Pos:  Pos{Line: 2, Col: 3},
					Map: []Pair{
						{
							Key:    "host",
							KeyPos: Pos{Line: 2, Col: 3},
							Value: &Node{
								Kind:   Scalar,
								Scalar: "localhost",
								Tag:    "!!str",
								Quoted: true,
								Pos:    Pos{Line: 2, Col: 9},
							},
						},
						{
							Key:    "port",
							KeyPos: Pos{Line: 3, Col: 3},
							Value: &Node{
								Kind:   Scalar,
								Scalar: "8080",
								Tag:    "!!int",
								Quoted: false,
								Pos:    Pos{Line: 3, Col: 9},
							},
						},
						{
							Key:    "tls",
							KeyPos: Pos{Line: 4, Col: 3},
							Value: &Node{
								Kind: Map,
								Pos:  Pos{Line: 5, Col: 5},
								Map: []Pair{
									{
										Key:    "enabled",
										KeyPos: Pos{Line: 5, Col: 5},
										Value: &Node{
											Kind:   Scalar,
											Scalar: "true",
											Tag:    "!!bool",
											Quoted: false,
											Pos:    Pos{Line: 5, Col: 14},
										},
									},
									{
										Key:    "cert",
										KeyPos: Pos{Line: 6, Col: 5},
										Value: &Node{
											Kind: Null,
											Pos:  Pos{Line: 6, Col: 11},
										},
									},
								},
							},
						},
						{
							Key:    "origins",
							KeyPos: Pos{Line: 7, Col: 3},
							Value: &Node{
								Kind: List,
								Pos:  Pos{Line: 8, Col: 5},
								List: []*Node{
									{
										Kind:   Scalar,
										Scalar: "https://a.example.com",
										Tag:    "!!str",
										Quoted: false,
										Pos:    Pos{Line: 8, Col: 7},
									},
									{
										Kind:   Scalar,
										Scalar: "https://b.example.com",
										Tag:    "!!str",
										Quoted: true,
										Pos:    Pos{Line: 9, Col: 7},
									},
									{
										Kind: Null,
										Pos:  Pos{Line: 10, Col: 6},
									},
								},
							},
						},
					},
				},
			},
			{
				Key:    "database",
				KeyPos: Pos{Line: 11, Col: 1},
				Value: &Node{
					Kind: Map,
					Pos:  Pos{Line: 12, Col: 3},
					Map: []Pair{
						{
							Key:    "name",
							KeyPos: Pos{Line: 12, Col: 3},
							Value: &Node{
								Kind:   Scalar,
								Scalar: "app",
								Tag:    "!!str",
								Quoted: false,
								Pos:    Pos{Line: 12, Col: 9},
							},
						},
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(root, want) {
		t.Fatalf("Parse tree mismatch:\ngot:  %#v\nwant: %#v", root, want)
	}

	// Test Node.Get.
	srv := root.Get("server")
	if srv == nil || srv.KeyPos != (Pos{Line: 1, Col: 1}) {
		t.Fatalf("root.Get(\"server\") = %+v", srv)
	}
	if got := srv.Value.Get("port"); got == nil || got.Value.Scalar != "8080" {
		t.Errorf("server.Get(\"port\") = %+v", got)
	}
	if got := root.Get("nonexistent"); got != nil {
		t.Errorf("root.Get(\"nonexistent\") = %+v, want nil", got)
	}
	var nilNode *Node
	if got := nilNode.Get("server"); got != nil {
		t.Errorf("nilNode.Get(\"server\") = %+v, want nil", got)
	}
	if got := srv.Value.Get("port").Value.Get("x"); got != nil {
		t.Errorf("scalarNode.Get(\"x\") = %+v, want nil", got)
	}
}

func TestParseJSONThroughYAMLPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, root *Node)
	}{
		{
			name:  "compact",
			input: `{"a":1,"b":{"c":[true,null,"ok"]}}`,
			check: func(t *testing.T, root *Node) {
				a := root.Get("a")
				if a == nil || a.KeyPos != (Pos{Line: 1, Col: 2}) || a.Value.Pos != (Pos{Line: 1, Col: 6}) || a.Value.Scalar != "1" || a.Value.Tag != "!!int" {
					t.Fatalf("a = %+v (val %+v)", a, a.Value)
				}
				b := root.Get("b")
				if b == nil || b.KeyPos != (Pos{Line: 1, Col: 8}) {
					t.Fatalf("b = %+v", b)
				}
				c := b.Value.Get("c")
				if c == nil || c.Value.Kind != List || len(c.Value.List) != 3 {
					t.Fatalf("c = %+v", c)
				}
				if c.Value.List[0].Scalar != "true" || c.Value.List[0].Tag != "!!bool" {
					t.Errorf("c[0] = %+v", c.Value.List[0])
				}
				if c.Value.List[1].Kind != Null {
					t.Errorf("c[1] = %+v, want Null", c.Value.List[1])
				}
				if c.Value.List[2].Scalar != "ok" || !c.Value.List[2].Quoted {
					t.Errorf("c[2] = %+v", c.Value.List[2])
				}
			},
		},
		{
			name: "indented_same_as_yaml",
			input: `{
  "server": {
    "host": "localhost",
    "port": 8080,
    "tls": {
      "enabled": true,
      "cert": null
    },
    "origins": [
      "https://a.example.com",
      "https://b.example.com"
    ]
  },
  "database": {
    "name": "app"
  }
}`,
			check: func(t *testing.T, root *Node) {
				srv := root.Get("server")
				if srv == nil || srv.KeyPos != (Pos{Line: 2, Col: 3}) || srv.Value.Pos != (Pos{Line: 2, Col: 13}) {
					t.Fatalf("server = %+v", srv)
				}
				port := srv.Value.Get("port")
				if port == nil || port.KeyPos != (Pos{Line: 4, Col: 5}) || port.Value.Pos != (Pos{Line: 4, Col: 13}) || port.Value.Scalar != "8080" || port.Value.Tag != "!!int" {
					t.Fatalf("port = %+v", port)
				}
				cert := srv.Value.Get("tls").Value.Get("cert")
				if cert == nil || cert.Value.Kind != Null || cert.Value.Pos != (Pos{Line: 7, Col: 15}) {
					t.Fatalf("cert = %+v", cert)
				}
				origins := srv.Value.Get("origins")
				if origins == nil || len(origins.Value.List) != 2 || origins.Value.List[0].Scalar != "https://a.example.com" {
					t.Fatalf("origins = %+v", origins)
				}
				db := root.Get("database").Value.Get("name")
				if db == nil || db.Value.Scalar != "app" || !db.Value.Quoted {
					t.Fatalf("database.name = %+v", db)
				}
			},
		},
		{
			name:  "escaped_slash",
			input: `{"url": "https:\/\/example.com\/v1\/health"}`,
			check: func(t *testing.T, root *Node) {
				u := root.Get("url")
				if u == nil || u.Value.Scalar != "https://example.com/v1/health" || u.Value.Tag != "!!str" || !u.Value.Quoted {
					t.Fatalf("url = %+v", u)
				}
			},
		},
		{
			name:  "tab_indentation",
			input: "{\n\t\"server\": {\n\t\t\"port\": 9090\n\t}\n}\n",
			check: func(t *testing.T, root *Node) {
				srv := root.Get("server")
				if srv == nil {
					t.Fatal("missing server")
				}
				port := srv.Value.Get("port")
				if port == nil || port.Value.Scalar != "9090" || port.Value.Tag != "!!int" {
					t.Fatalf("port = %+v", port)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, err := Parse("config.json", []byte(tc.input), JSON)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			tc.check(t, root)
		})
	}
}

func TestScalarsTagsAndQuotes(t *testing.T) {
	const src = `yaml11_yes: yes
yaml11_no: no
yaml11_on: on
yaml11_off: off
yaml11_y: y
yaml11_n: n
octal_legacy: 0777
octal_modern: 0o777
hex_int: 0x10
underscore_int: 1_000
plain_float: 1.0
inf_float: .inf
bool_true: true
bool_false: false
timestamp: 2026-11-08
plain_str: hello
single_quoted: 'hello'
double_quoted: "1.0"
literal_block: |
  line1
  line2
folded_block: >
  line1
  line2
null_tilde: ~
null_word: null
null_empty:
quoted_null: "null"
quoted_tilde: '~'
custom_tag: !custom myval
`
	root, err := Parse("scalars.yaml", []byte(src), YAML)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	tests := []struct {
		key    string
		kind   Kind
		scalar string
		tag    string
		quoted bool
	}{
		{key: "yaml11_yes", kind: Scalar, scalar: "yes", tag: "!!str", quoted: false},
		{key: "yaml11_no", kind: Scalar, scalar: "no", tag: "!!str", quoted: false},
		{key: "yaml11_on", kind: Scalar, scalar: "on", tag: "!!str", quoted: false},
		{key: "yaml11_off", kind: Scalar, scalar: "off", tag: "!!str", quoted: false},
		{key: "yaml11_y", kind: Scalar, scalar: "y", tag: "!!str", quoted: false},
		{key: "yaml11_n", kind: Scalar, scalar: "n", tag: "!!str", quoted: false},
		{key: "octal_legacy", kind: Scalar, scalar: "0777", tag: "!!int", quoted: false},
		{key: "octal_modern", kind: Scalar, scalar: "0o777", tag: "!!int", quoted: false},
		{key: "hex_int", kind: Scalar, scalar: "0x10", tag: "!!int", quoted: false},
		{key: "underscore_int", kind: Scalar, scalar: "1_000", tag: "!!int", quoted: false},
		{key: "plain_float", kind: Scalar, scalar: "1.0", tag: "!!float", quoted: false},
		{key: "inf_float", kind: Scalar, scalar: ".inf", tag: "!!float", quoted: false},
		{key: "bool_true", kind: Scalar, scalar: "true", tag: "!!bool", quoted: false},
		{key: "bool_false", kind: Scalar, scalar: "false", tag: "!!bool", quoted: false},
		{key: "timestamp", kind: Scalar, scalar: "2026-11-08", tag: "!!timestamp", quoted: false},
		{key: "plain_str", kind: Scalar, scalar: "hello", tag: "!!str", quoted: false},
		{key: "single_quoted", kind: Scalar, scalar: "hello", tag: "!!str", quoted: true},
		{key: "double_quoted", kind: Scalar, scalar: "1.0", tag: "!!str", quoted: true},
		{key: "literal_block", kind: Scalar, scalar: "line1\nline2\n", tag: "!!str", quoted: true},
		{key: "folded_block", kind: Scalar, scalar: "line1 line2\n", tag: "!!str", quoted: true},
		{key: "null_tilde", kind: Null, scalar: "", tag: "", quoted: false},
		{key: "null_word", kind: Null, scalar: "", tag: "", quoted: false},
		{key: "null_empty", kind: Null, scalar: "", tag: "", quoted: false},
		{key: "quoted_null", kind: Scalar, scalar: "null", tag: "!!str", quoted: true},
		{key: "quoted_tilde", kind: Scalar, scalar: "~", tag: "!!str", quoted: true},
		{key: "custom_tag", kind: Scalar, scalar: "myval", tag: "!custom", quoted: false},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			p := root.Get(tc.key)
			if p == nil {
				t.Fatalf("missing key %q", tc.key)
			}
			v := p.Value
			if v.Kind != tc.kind || v.Scalar != tc.scalar || v.Tag != tc.tag || v.Quoted != tc.quoted {
				t.Errorf("%s = {Kind:%v Scalar:%q Tag:%q Quoted:%v}, want {Kind:%v Scalar:%q Tag:%q Quoted:%v}",
					tc.key, v.Kind, v.Scalar, v.Tag, v.Quoted, tc.kind, tc.scalar, tc.tag, tc.quoted)
			}
		})
	}
}

func TestEmptyAndNullRoots(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		input  string
	}{
		{name: "empty", format: YAML, input: ""},
		{name: "whitespace_only", format: YAML, input: "   \n\t\n"},
		{name: "comment_only", format: YAML, input: "# comment line 1\n# comment line 2\n"},
		{name: "doc_start_only", format: YAML, input: "---\n"},
		{name: "null_word", format: YAML, input: "null\n"},
		{name: "null_tilde", format: YAML, input: "~\n"},
		{name: "comment_then_null", format: YAML, input: "# header\n~\n"},
		{name: "json_null", format: JSON, input: "null"},
		{name: "empty_flow_map", format: JSON, input: "{}"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, err := Parse("empty.yaml", []byte(tc.input), tc.format)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tc.input, err)
			}
			want := &Node{Kind: Map, Pos: Pos{Line: 1, Col: 1}}
			if !reflect.DeepEqual(root, want) {
				t.Errorf("Parse(%q) = %#v, want %#v", tc.input, root, want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		format  Format
		input   string
		wantPos Pos
		wantMsg string
	}{
		{
			name:    "scalar_root_yaml",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "hello\n",
			wantPos: Pos{Line: 1, Col: 1},
			wantMsg: "the top level must be a map of settings",
		},
		{
			name:    "quoted_null_root_yaml",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "\"null\"\n",
			wantPos: Pos{Line: 1, Col: 1},
			wantMsg: "the top level must be a map of settings",
		},
		{
			name:    "list_root_yaml",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "# comment\n- a\n- b\n",
			wantPos: Pos{Line: 2, Col: 1},
			wantMsg: "the top level must be a map of settings",
		},
		{
			name:    "list_root_json",
			file:    "cfg.json",
			format:  JSON,
			input:   "[1, 2, 3]",
			wantPos: Pos{Line: 1, Col: 1},
			wantMsg: "the top level must be a map of settings",
		},
		{
			name:    "duplicate_key_yaml_top",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "a: 1\nb: 2\na: 3\n",
			wantPos: Pos{Line: 3, Col: 1},
			wantMsg: `duplicate key "a" (first at 1:1)`,
		},
		{
			name:    "duplicate_key_yaml_nested",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "server:\n  port: 80\n  host: localhost\n  port: 443\n",
			wantPos: Pos{Line: 4, Col: 3},
			wantMsg: `duplicate key "port" (first at 2:3)`,
		},
		{
			name:    "duplicate_key_json",
			file:    "cfg.json",
			format:  JSON,
			input:   "{\n  \"x\": 1,\n  \"y\": 2,\n  \"x\": 3\n}\n",
			wantPos: Pos{Line: 4, Col: 3},
			wantMsg: `duplicate key "x" (first at 2:3)`,
		},
		{
			name:    "non_scalar_key_seq",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "? [a, b]\n: 1\n",
			wantPos: Pos{Line: 1, Col: 3},
			wantMsg: "map keys must be scalars",
		},
		{
			name:    "non_scalar_key_map",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "? {k: v}\n: 1\n",
			wantPos: Pos{Line: 1, Col: 3},
			wantMsg: "map keys must be scalars",
		},
		{
			name:    "anchor_on_root",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "--- &root\na: 1\n",
			wantPos: Pos{Line: 1, Col: 5},
			wantMsg: "anchors and aliases aren't supported",
		},
		{
			name:    "anchor_on_value",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "a: &anc 1\n",
			wantPos: Pos{Line: 1, Col: 4},
			wantMsg: "anchors and aliases aren't supported",
		},
		{
			name:    "anchor_on_key",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "&anc a: 1\n",
			wantPos: Pos{Line: 1, Col: 1},
			wantMsg: "anchors and aliases aren't supported",
		},
		{
			name:    "anchor_in_list",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "items:\n  - &item one\n",
			wantPos: Pos{Line: 2, Col: 5},
			wantMsg: "anchors and aliases aren't supported",
		},
		{
			name:    "merge_key_unquoted",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "child:\n  <<: {x: 1}\n",
			wantPos: Pos{Line: 2, Col: 3},
			wantMsg: "merge keys (<<) aren't supported",
		},
		{
			name:    "multi_document",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "a: 1\n---\nb: 2\n",
			wantPos: Pos{},
			wantMsg: "expected single document, found multiple",
		},
		{
			name:    "syntax_error_position",
			file:    "cfg.yaml",
			format:  YAML,
			input:   "a: 1\nb: [unclosed\n",
			wantPos: Pos{Line: 3, Col: 1},
			wantMsg: "did not find expected ',' or ']'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.file, []byte(tc.input), tc.format)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var nodeErr *Error
			if !errors.As(err, &nodeErr) {
				t.Fatalf("error %T is not *Error: %v", err, err)
			}
			if nodeErr.File != tc.file {
				t.Errorf("Error.File = %q, want %q", nodeErr.File, tc.file)
			}
			if nodeErr.Pos != tc.wantPos {
				t.Errorf("Error.Pos = %+v, want %+v (err: %v)", nodeErr.Pos, tc.wantPos, nodeErr)
			}
			if !strings.Contains(nodeErr.Msg, tc.wantMsg) {
				t.Errorf("Error.Msg = %q, want substring %q", nodeErr.Msg, tc.wantMsg)
			}
			if strings.Contains(nodeErr.Msg, "go-yaml load error") {
				t.Errorf("Error.Msg still contains go-yaml prefix: %q", nodeErr.Msg)
			}
		})
	}
}

func TestAliasNodeAndQuotedMergeKey(t *testing.T) {
	// Quoted "<<" is an ordinary key, not a merge key.
	_, err := Parse("cfg.yaml", []byte("\"<<\": literal\n'<<': second\n"), YAML)
	var nodeErr *Error
	if !errors.As(err, &nodeErr) || nodeErr.Pos != (Pos{Line: 2, Col: 1}) {
		t.Fatalf("expected duplicate key error at 2:1 on quoted '<<', got %v", err)
	}

	root, err := Parse("cfg.yaml", []byte("\"<<\": literal\n"), YAML)
	if err != nil {
		t.Fatalf("Parse quoted '<<' failed: %v", err)
	}
	p := root.Get("<<")
	if p == nil || p.Value.Scalar != "literal" || p.Value.Tag != "!!str" || p.Value.Quoted {
		t.Fatalf("Get(\"<<\") = %+v (val %+v)", p, p.Value)
	}

	// Verify AliasNode is rejected at its position both as a value and as a key.
	aliasValDoc := &yaml.Node{
		Kind:   yaml.MappingNode,
		Line:   1,
		Column: 1,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "k", Tag: "!!str", Line: 1, Column: 1},
			{Kind: yaml.AliasNode, Value: "a", Line: 1, Column: 4},
		},
	}
	_, err = walk("alias.yaml", aliasValDoc)
	if !errors.As(err, &nodeErr) || nodeErr.Pos != (Pos{Line: 1, Col: 4}) || nodeErr.Msg != "anchors and aliases aren't supported" {
		t.Errorf("walk(AliasNode value) = %v, want anchors and aliases error at 1:4", err)
	}

	aliasKeyDoc := &yaml.Node{
		Kind:   yaml.MappingNode,
		Line:   1,
		Column: 1,
		Content: []*yaml.Node{
			{Kind: yaml.AliasNode, Value: "a", Line: 2, Column: 3},
			{Kind: yaml.ScalarNode, Value: "v", Tag: "!!str", Line: 2, Column: 7},
		},
	}
	_, err = walk("alias.yaml", aliasKeyDoc)
	if !errors.As(err, &nodeErr) || nodeErr.Pos != (Pos{Line: 2, Col: 3}) || nodeErr.Msg != "anchors and aliases aren't supported" {
		t.Errorf("walk(AliasNode key) = %v, want anchors and aliases error at 2:3", err)
	}
}

func TestJSONSurrogates(t *testing.T) {
	// If nothing changed, rewriteSurrogates returns the input slice without copying.
	unchanged := []byte(`{"plain": "hello\\uD83D\\uDE00", "num": 42}`)
	gotSlice := rewriteSurrogates(unchanged)
	if &gotSlice[0] != &unchanged[0] {
		t.Errorf("rewriteSurrogates copied input when no surrogates were present")
	}

	// Outside strings nothing is rewritten.
	outside := []byte("# \\uD83D\\uDE00\n{\"a\": 1}\n")
	gotOutside := rewriteSurrogates(outside)
	if &gotOutside[0] != &outside[0] {
		t.Errorf("rewriteSurrogates modified bytes outside strings")
	}

	tests := []struct {
		name        string
		input       string
		firstKey    string
		wantScalar  string
		wantNextPos Pos
		wantValPos  Pos
	}{
		{
			name:        "pair_in_value",
			input:       `{"a": "\uD83D\uDE00", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "😀",
			wantNextPos: Pos{Line: 1, Col: 23},
			wantValPos:  Pos{Line: 1, Col: 28},
		},
		{
			name:        "pair_in_key",
			input:       `{"\uD83D\uDE00": 1, "b": 2}`,
			firstKey:    "😀",
			wantScalar:  "1",
			wantNextPos: Pos{Line: 1, Col: 21},
			wantValPos:  Pos{Line: 1, Col: 26},
		},
		{
			name:        "two_pairs_in_one_string",
			input:       `{"a": "\uD83D\uDE00-\uD83D\uDE80", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "😀-🚀",
			wantNextPos: Pos{Line: 1, Col: 36},
			wantValPos:  Pos{Line: 1, Col: 41},
		},
		{
			name:        "lone_high",
			input:       `{"a": "\uD83D", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "\uFFFD",
			wantNextPos: Pos{Line: 1, Col: 17},
			wantValPos:  Pos{Line: 1, Col: 22},
		},
		{
			name:        "lone_low",
			input:       `{"a": "\uDE00", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "\uFFFD",
			wantNextPos: Pos{Line: 1, Col: 17},
			wantValPos:  Pos{Line: 1, Col: 22},
		},
		{
			name:        "high_followed_by_non_surrogate_u_escape",
			input:       `{"a": "\uD83D\u0041", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "\uFFFDA",
			wantNextPos: Pos{Line: 1, Col: 23},
			wantValPos:  Pos{Line: 1, Col: 28},
		},
		{
			name:        "high_followed_by_short_escape",
			input:       `{"a": "\uD83D\n", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "\uFFFD\n",
			wantNextPos: Pos{Line: 1, Col: 19},
			wantValPos:  Pos{Line: 1, Col: 24},
		},
		{
			name:        "lone_high_before_valid_pair",
			input:       `{"a": "\uD800\uD83D\uDE00", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "\uFFFD😀",
			wantNextPos: Pos{Line: 1, Col: 29},
			wantValPos:  Pos{Line: 1, Col: 34},
		},
		{
			name:        "escaped_backslash_before_u",
			input:       `{"a": "\\uD83D\\uDE00", "b": 2}`,
			firstKey:    "a",
			wantScalar:  `\uD83D\uDE00`,
			wantNextPos: Pos{Line: 1, Col: 25},
			wantValPos:  Pos{Line: 1, Col: 30},
		},
		{
			name:        "lowercase_hex",
			input:       `{"a": "\ud83d\ude00", "b": 2}`,
			firstKey:    "a",
			wantScalar:  "😀",
			wantNextPos: Pos{Line: 1, Col: 23},
			wantValPos:  Pos{Line: 1, Col: 28},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, err := Parse("cfg.json", []byte(tc.input), JSON)
			if err != nil {
				t.Fatalf("Parse(%s) failed: %v", tc.input, err)
			}
			first := root.Get(tc.firstKey)
			if first == nil {
				t.Fatalf("missing key %q in %+v", tc.firstKey, root.Map)
			}
			if first.Value.Scalar != tc.wantScalar {
				t.Errorf("first.Value.Scalar = %q, want %q", first.Value.Scalar, tc.wantScalar)
			}
			b := root.Get("b")
			if b == nil {
				t.Fatal("missing key \"b\"")
			}
			if b.KeyPos != tc.wantNextPos {
				t.Errorf("b.KeyPos = %+v, want %+v", b.KeyPos, tc.wantNextPos)
			}
			if b.Value.Pos != tc.wantValPos {
				t.Errorf("b.Value.Pos = %+v, want %+v", b.Value.Pos, tc.wantValPos)
			}
		})
	}

	// Verify surrogate rewrite is NOT applied when f == YAML.
	if _, err := Parse("cfg.yaml", []byte("a: \"\\uD83D\\uDE00\"\n"), YAML); err == nil {
		t.Errorf("expected YAML format to reject UTF-16 surrogate escapes")
	}
}
