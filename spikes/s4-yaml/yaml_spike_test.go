package s4yaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	goccy "github.com/goccy/go-yaml"
	goccyast "github.com/goccy/go-yaml/ast"
	goccyparser "github.com/goccy/go-yaml/parser"
	yamlv3 "go.yaml.in/yaml/v3"
	yamlv4 "go.yaml.in/yaml/v4"
	gopkgv3 "gopkg.in/yaml.v3"
)

type candidateParser struct {
	name  string
	parse func([]byte) (*Node, error)
}

func allCandidates() []candidateParser {
	return []candidateParser{
		{name: "go.yaml.in/yaml/v4", parse: ParseV4},
		{name: "go.yaml.in/yaml/v3", parse: ParseV3},
		{name: "github.com/goccy/go-yaml", parse: ParseGoccy},
		{name: "gopkg.in/yaml.v3", parse: ParseGopkgV3},
	}
}

// -----------------------------------------------------------------------------
// Criterion 1: Node API with positions and original scalar text
// -----------------------------------------------------------------------------

func TestCriterion1_NodeAPIPositionsAndRawScalars(t *testing.T) {
	input := strings.Join([]string{
		`server:`,             // line 1
		`  host: "localhost"`, // line 2: key 2:3, val 2:9
		`  port: 8080`,        // line 3: key 3:3, val 3:9
		`  ratios:`,           // line 4: key 4:3
		`    - 1.0`,           // line 5: elem 5:7
		`    - "1.0"`,         // line 6: elem 6:7
		`    - 1`,             // line 7: elem 7:7
		`    - 0x10`,          // line 8: elem 8:7
		`    - "0x10"`,        // line 9: elem 9:7
		`    - yes`,           // line 10: elem 10:7
		`    - "yes"`,         // line 11: elem 11:7
	}, "\n") + "\n"

	for _, c := range allCandidates() {
		t.Run(c.name, func(t *testing.T) {
			root, err := c.parse([]byte(input))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			srvEntry, ok := root.LookupEntry("server")
			if !ok {
				t.Fatalf("missing server key")
			}
			if srvEntry.KeyPos != (Pos{Line: 1, Column: 1}) {
				t.Errorf("server KeyPos = %+v, want {1, 1}", srvEntry.KeyPos)
			}

			hostEntry, ok := root.LookupEntry("server", "host")
			if !ok {
				t.Fatalf("missing server.host")
			}
			if hostEntry.KeyPos != (Pos{Line: 2, Column: 3}) {
				t.Errorf("host KeyPos = %+v, want {2, 3}", hostEntry.KeyPos)
			}
			if hostEntry.Value.Pos != (Pos{Line: 2, Column: 9}) {
				t.Errorf("host Value.Pos = %+v, want {2, 9}", hostEntry.Value.Pos)
			}
			if hostEntry.Value.Scalar != "localhost" || !hostEntry.Value.Quoted {
				t.Errorf("host Value = %+v, want quoted localhost", hostEntry.Value)
			}

			portEntry, ok := root.LookupEntry("server", "port")
			if !ok {
				t.Fatalf("missing server.port")
			}
			if portEntry.KeyPos != (Pos{Line: 3, Column: 3}) || portEntry.Value.Pos != (Pos{Line: 3, Column: 9}) {
				t.Errorf("port pos = key %+v val %+v, want {3,3} and {3,9}", portEntry.KeyPos, portEntry.Value.Pos)
			}
			if portEntry.Value.Scalar != "8080" || portEntry.Value.Tag != "!!int" || portEntry.Value.Quoted {
				t.Errorf("port Value = %+v, want unquoted !!int 8080", portEntry.Value)
			}

			ratios, ok := root.Lookup("server", "ratios")
			if !ok || ratios.Kind != KindSeq || len(ratios.List) != 7 {
				t.Fatalf("ratios = %+v, want sequence of 7 items", ratios)
			}

			wantItems := []struct {
				scalar string
				tag    string
				quoted bool
				line   int
				col    int
			}{
				{scalar: "1.0", tag: "!!float", quoted: false, line: 5, col: 7},
				{scalar: "1.0", tag: "!!str", quoted: true, line: 6, col: 7},
				{scalar: "1", tag: "!!int", quoted: false, line: 7, col: 7},
				{scalar: "0x10", tag: "!!int", quoted: false, line: 8, col: 7},
				{scalar: "0x10", tag: "!!str", quoted: true, line: 9, col: 7},
				{scalar: "yes", tag: "!!str", quoted: false, line: 10, col: 7},
				{scalar: "yes", tag: "!!str", quoted: true, line: 11, col: 7},
			}

			for i, want := range wantItems {
				got := ratios.List[i]
				if got.Scalar != want.scalar || got.Tag != want.tag || got.Quoted != want.quoted {
					t.Errorf("ratios[%d] = {Scalar:%q Tag:%s Quoted:%v}, want {Scalar:%q Tag:%s Quoted:%v}",
						i, got.Scalar, got.Tag, got.Quoted, want.scalar, want.tag, want.quoted)
				}
				if got.Pos != (Pos{Line: want.line, Column: want.col}) {
					t.Errorf("ratios[%d].Pos = %+v, want {%d, %d}", i, got.Pos, want.line, want.col)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Criterion 2: Strict duplicate keys (YAML and JSON)
// -----------------------------------------------------------------------------

func TestCriterion2_DuplicateKeys(t *testing.T) {
	yamlDup := "a: 1\nb: 2\na: 3\n"
	jsonDup := "{\n  \"a\": 1,\n  \"b\": 2,\n  \"a\": 3\n}\n"

	t.Run("RawNodeLoadBypassesConstructorUniqueKeyCheck", func(t *testing.T) {
		// Documenting a critical finding: in go.yaml.in/yaml/v4, go.yaml.in/yaml/v3,
		// and gopkg.in/yaml.v3, Constructor.checkMappingKeys (WithUniqueKeys(true)) is
		// skipped when the target type is yaml.Node (it returns the raw DocumentNode
		// directly containing all duplicate key-value pairs in MappingNode.Content).
		// When decoding into `any` (or a struct/map), all three reject duplicates by default.
		var docV4 yamlv4.Node
		if err := yamlv4.Load([]byte(yamlDup), &docV4); err != nil {
			t.Fatalf("expected raw yamlv4.Node Load to succeed, got %v", err)
		}
		if got := len(docV4.Content[0].Content); got != 6 {
			t.Fatalf("expected 6 nodes (3 key-value pairs) in raw MappingNode.Content, got %d", got)
		}

		var anyV4 any
		errV4 := yamlv4.Load([]byte(yamlDup), &anyV4)
		if errV4 == nil || !strings.Contains(errV4.Error(), `mapping key "a" already defined at line 1`) {
			t.Fatalf("expected v4 Load into any to reject duplicate key, got %v", errV4)
		}
		var loadErrs *yamlv4.LoadErrors
		if !errors.As(errV4, &loadErrs) || len(loadErrs.Errors) != 1 {
			t.Fatalf("expected *yamlv4.LoadErrors with 1 error, got %T %v", errV4, errV4)
		}
		if loadErrs.Errors[0].Mark.Line != 3 || loadErrs.Errors[0].Mark.Column != 1 {
			t.Errorf("v4 LoadError Mark = L%d.C%d, want L3.C1", loadErrs.Errors[0].Mark.Line, loadErrs.Errors[0].Mark.Column)
		}

		var anyV3 any
		if err := yamlv3.Unmarshal([]byte(yamlDup), &anyV3); err == nil || !strings.Contains(err.Error(), `mapping key "a" already defined at line 1`) {
			t.Fatalf("expected v3 Unmarshal into any to reject duplicate key, got %v", err)
		}
		var anyGopkg any
		if err := gopkgv3.Unmarshal([]byte(yamlDup), &anyGopkg); err == nil || !strings.Contains(err.Error(), `mapping key "a" already defined at line 1`) {
			t.Fatalf("expected gopkg.v3 Unmarshal into any to reject duplicate key, got %v", err)
		}
	})

	t.Run("NodeWalkDetectsDuplicatesWithLineAndCol", func(t *testing.T) {
		for _, c := range allCandidates() {
			t.Run(c.name, func(t *testing.T) {
				_, errYAML := c.parse([]byte(yamlDup))
				if errYAML == nil {
					t.Fatalf("expected error on YAML duplicate key")
				}
				// Both 3:1 and 1:1 are reported.
				if !strings.Contains(errYAML.Error(), "3:1") || !strings.Contains(errYAML.Error(), "1:1") {
					t.Errorf("YAML duplicate error %q missing expected positions 3:1 and 1:1", errYAML.Error())
				}

				_, errJSON := c.parse([]byte(jsonDup))
				if errJSON == nil {
					t.Fatalf("expected error on JSON duplicate key")
				}
				if !strings.Contains(errJSON.Error(), "4:3") || !strings.Contains(errJSON.Error(), "2:3") {
					t.Errorf("JSON duplicate error %q missing expected positions 4:3 and 2:3", errJSON.Error())
				}
			})
		}
	})

	t.Run("GoccyFalsePositiveDuplicateKeyPathCollisionBug", func(t *testing.T) {
		// In github.com/goccy/go-yaml v1.19.2, parser/context.go normalizes path
		// segments containing '.', '$', '*', '[', ']' by wrapping them in single
		// quotes without escaping existing single quotes, and checks duplicate keys
		// against a single document-wide flat map `p.pathMap[keyPath]`.
		// As a result, two completely distinct keys at different depths collide on
		// the path `$.'a'.'b'` and trigger a false-positive duplicate key error!
		validJSON := "{\n  \"'a'\": {\"'b'\": 1},\n  \"a'.'b\": 2\n}\n"

		// Validated by encoding/json, go.yaml.in/yaml/v4, and go.yaml.in/yaml/v3:
		var stdMap map[string]any
		if err := json.Unmarshal([]byte(validJSON), &stdMap); err != nil {
			t.Fatalf("encoding/json rejected valid JSON: %v", err)
		}
		if _, err := ParseV4([]byte(validJSON)); err != nil {
			t.Fatalf("ParseV4 rejected valid JSON: %v", err)
		}
		if _, err := ParseV3([]byte(validJSON)); err != nil {
			t.Fatalf("ParseV3 rejected valid JSON: %v", err)
		}

		// goccy/go-yaml falsely fails with duplicate mapping key error:
		_, goccyErr := ParseGoccy([]byte(validJSON))
		if goccyErr == nil || !strings.Contains(goccyErr.Error(), `mapping key "a'.'b" already defined`) {
			t.Fatalf("expected goccy false-positive duplicate key collision, got: %v", goccyErr)
		}
	})
}

// -----------------------------------------------------------------------------
// Criterion 3: JSON input through the YAML parser & divergences from encoding/json
// -----------------------------------------------------------------------------

func TestCriterion3_JSONThroughYAML(t *testing.T) {
	// Standard JSON payload covering nested objects/arrays, compact colons, escapes
	// (literal UTF-8 é, \u00e9, \", \\, \b, \f, \n, \r, \t), ints > 2^63-1,
	// floats (including scientific notation), true/false/null, empty {} and [].
	// Note: escaped forward slash (\/) is tested in its own subtest below because
	// go.yaml.in/yaml/v3 and gopkg.in/yaml.v3 reject \/ with "found unknown escape character"!
	jsonPayload := `{
	"nested": {"inner": {"arr": [1, 2, {"deep": true}]}},
	"compact":{"no_space":1,"url":"https://example.com:8080/a/b"},
	"escapes": "é \u00e9 \" \\ \b \f \n \r \t",
	"big_uint64": 9223372036854775808,
	"max_uint64": 18446744073709551615,
	"beyond_uint64": 18446744073709551616,
	"floats": [3.14159, -0.001, 1e10, 1e-5, 2.5e+3],
	"booleans": [true, false],
	"null_val": null,
	"empty_obj": {},
	"empty_arr": []
}`

	var stdAny map[string]any
	dec := json.NewDecoder(strings.NewReader(jsonPayload))
	dec.UseNumber()
	if err := dec.Decode(&stdAny); err != nil {
		t.Fatalf("encoding/json failed: %v", err)
	}
	wantEscapes := stdAny["escapes"].(string)

	for _, c := range allCandidates() {
		t.Run(c.name, func(t *testing.T) {
			root, err := c.parse([]byte(jsonPayload))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			// Nested object + array
			deep, ok := root.Lookup("nested", "inner", "arr")
			if !ok || deep.Kind != KindSeq || len(deep.List) != 3 {
				t.Fatalf("nested.inner.arr = %+v", deep)
			}
			deepFlag, ok := deep.List[2].Lookup("deep")
			if !ok || deepFlag.Scalar != "true" || deepFlag.Tag != "!!bool" {
				t.Errorf("deep = %+v, want !!bool true", deepFlag)
			}

			// Compact JSON without space after ':'
			urlNode, ok := root.Lookup("compact", "url")
			if !ok || urlNode.Scalar != "https://example.com:8080/a/b" {
				t.Errorf("compact.url = %+v", urlNode)
			}

			// Escapes match encoding/json byte-for-byte
			escNode, ok := root.Lookup("escapes")
			if !ok || escNode.Scalar != wantEscapes {
				t.Errorf("escapes = %q, want %q", escNode.Scalar, wantEscapes)
			}

			// Integers > 2^63-1 preserve exact raw digits
			bigUint, _ := root.Lookup("big_uint64")
			if bigUint.Scalar != "9223372036854775808" || bigUint.Tag != "!!int" {
				t.Errorf("big_uint64 = %+v, want !!int 9223372036854775808", bigUint)
			}
			maxUint, _ := root.Lookup("max_uint64")
			if maxUint.Scalar != "18446744073709551615" || maxUint.Tag != "!!int" {
				t.Errorf("max_uint64 = %+v, want !!int 18446744073709551615", maxUint)
			}
			beyondUint, _ := root.Lookup("beyond_uint64")
			if beyondUint.Scalar != "18446744073709551616" {
				t.Errorf("beyond_uint64 Scalar = %q, want 18446744073709551616", beyondUint.Scalar)
			}
			// Divergence in tag for >= 2^64:
			if c.name == "github.com/goccy/go-yaml" {
				if beyondUint.Tag != "!!str" {
					t.Errorf("goccy beyond_uint64 Tag = %s, want !!str", beyondUint.Tag)
				}
			} else {
				if beyondUint.Tag != "!!float" {
					t.Errorf("%s beyond_uint64 Tag = %s, want !!float", c.name, beyondUint.Tag)
				}
			}

			// Floats and scientific notation
			floats, _ := root.Lookup("floats")
			if len(floats.List) != 5 {
				t.Fatalf("floats len = %d, want 5", len(floats.List))
			}
			if c.name == "github.com/goccy/go-yaml" {
				// Shortcoming in goccy/go-yaml: scientific-notation numbers without a '.'
				// (like 1e10 and 1e-5) fail token.toNumber (which checks strings.Count(v, ".") == 0
				// and calls strconv.ParseUint) and become StringNode (!!str) instead of FloatNode!
				if floats.List[2].Tag != "!!str" || floats.List[3].Tag != "!!str" {
					t.Errorf("expected goccy 1e10/1e-5 to be misclassified as !!str, got %s and %s",
						floats.List[2].Tag, floats.List[3].Tag)
				}
			} else {
				for i, fn := range floats.List {
					if fn.Tag != "!!float" {
						t.Errorf("floats[%d] (%q) Tag = %s, want !!float", i, fn.Scalar, fn.Tag)
					}
				}
			}

			// Booleans, null, empty object & array
			bools, _ := root.Lookup("booleans")
			if bools.List[0].Scalar != "true" || bools.List[0].Tag != "!!bool" ||
				bools.List[1].Scalar != "false" || bools.List[1].Tag != "!!bool" {
				t.Errorf("booleans = %+v", bools.List)
			}
			nullVal, _ := root.Lookup("null_val")
			if nullVal.Kind != KindNull || nullVal.Scalar != "null" || nullVal.Tag != "!!null" {
				t.Errorf("null_val = %+v", nullVal)
			}
			emptyObj, _ := root.Lookup("empty_obj")
			if emptyObj.Kind != KindMap || len(emptyObj.Map) != 0 {
				t.Errorf("empty_obj = %+v", emptyObj)
			}
			emptyArr, _ := root.Lookup("empty_arr")
			if emptyArr.Kind != KindSeq || len(emptyArr.List) != 0 {
				t.Errorf("empty_arr = %+v", emptyArr)
			}
		})
	}

	t.Run("Divergence_EscapedForwardSlash", func(t *testing.T) {
		// RFC 8259 §7 and YAML 1.2 §5.7 allow `\/` inside double-quoted strings,
		// whereas YAML 1.1 did not.
		// go.yaml.in/yaml/v4 (internal/libyaml/scanner.go:2345) and goccy/go-yaml
		// accept `\/` and decode it to `/`.
		// go.yaml.in/yaml/v3 (scannerc.go:2550) and gopkg.in/yaml.v3 REJECT `\/`
		// with "yaml: found unknown escape character"!
		slashJSON := `{"path": "https:\/\/example.com\/a\/b"}`

		var stdMap map[string]string
		if err := json.Unmarshal([]byte(slashJSON), &stdMap); err != nil || stdMap["path"] != "https://example.com/a/b" {
			t.Fatalf("encoding/json failed on \\/: %v %v", err, stdMap)
		}

		// yaml/v4 and goccy/go-yaml succeed:
		for _, c := range []candidateParser{
			{name: "go.yaml.in/yaml/v4", parse: ParseV4},
			{name: "github.com/goccy/go-yaml", parse: ParseGoccy},
		} {
			root, err := c.parse([]byte(slashJSON))
			if err != nil {
				t.Fatalf("%s rejected JSON \\/ escape: %v", c.name, err)
			}
			p, ok := root.Lookup("path")
			if !ok || p.Scalar != "https://example.com/a/b" {
				t.Errorf("%s path = %+v, want https://example.com/a/b", c.name, p)
			}
		}

		// yaml/v3 and gopkg.in/yaml.v3 fail with "found unknown escape character":
		for _, c := range []candidateParser{
			{name: "go.yaml.in/yaml/v3", parse: ParseV3},
			{name: "gopkg.in/yaml.v3", parse: ParseGopkgV3},
		} {
			_, err := c.parse([]byte(slashJSON))
			if err == nil || !strings.Contains(err.Error(), "found unknown escape character") {
				t.Errorf("expected %s to fail with 'found unknown escape character' on \\/, got: %v", c.name, err)
			}
		}
	})

	t.Run("Divergence_UTF16SurrogatePairs", func(t *testing.T) {
		// RFC 8259 §7 allows supplementary characters (U+10000..U+10FFFF) to be
		// escaped as a 12-character UTF-16 surrogate pair (\uD83D\uDE00 = 😀).
		// YAML 1.2 §5.7 instead uses \U0001F600 (8 hex digits) and forbids
		// UTF-16 surrogate code points (U+D800..U+DFFF) in \uXXXX escapes.
		surrogateJSON := `{"emoji": "\uD83D\uDE00"}`

		var stdMap map[string]string
		if err := json.Unmarshal([]byte(surrogateJSON), &stdMap); err != nil || stdMap["emoji"] != "😀" {
			t.Fatalf("encoding/json failed on surrogate pair: %v %v", err, stdMap)
		}

		// yaml/v4, yaml/v3, and gopkg.in/yaml.v3 reject \uD83D\uDE00:
		if _, err := ParseV4([]byte(surrogateJSON)); err == nil {
			t.Errorf("expected yaml/v4 to reject UTF-16 surrogate escape \\uD83D\\uDE00")
		}
		if _, err := ParseV3([]byte(surrogateJSON)); err == nil {
			t.Errorf("expected yaml/v3 to reject UTF-16 surrogate escape \\uD83D\\uDE00")
		}
		if _, err := ParseGopkgV3([]byte(surrogateJSON)); err == nil {
			t.Errorf("expected gopkg.in/yaml.v3 to reject UTF-16 surrogate escape \\uD83D\\uDE00")
		}

		// Literal UTF-8 emoji in JSON works in all parsers:
		utf8EmojiJSON := `{"emoji": "😀"}`
		for _, c := range allCandidates() {
			n, err := c.parse([]byte(utf8EmojiJSON))
			if err != nil {
				t.Fatalf("%s rejected literal UTF-8 emoji: %v", c.name, err)
			}
			em, _ := n.Lookup("emoji")
			if em.Scalar != "😀" {
				t.Errorf("%s emoji = %q, want 😀", c.name, em.Scalar)
			}
		}

		// goccy/go-yaml decodes \uD83D\uDE00 to 😀:
		goccyNode, err := ParseGoccy([]byte(surrogateJSON))
		if err != nil {
			t.Fatalf("goccy rejected surrogate pair: %v", err)
		}
		if em, _ := goccyNode.Lookup("emoji"); em.Scalar != "😀" {
			t.Errorf("goccy surrogate emoji = %q, want 😀", em.Scalar)
		}
	})

	t.Run("Divergence_YAMLSupersetAcceptedAsJSON", func(t *testing.T) {
		// All YAML parsers accept YAML superset constructs (comments, trailing commas,
		// unquoted keys) that encoding/json rejects.
		superset := "{\n  # YAML comment inside JSON\n  unquoted_key: \"val\",\n}\n"
		if err := json.Unmarshal([]byte(superset), &map[string]any{}); err == nil {
			t.Fatalf("expected encoding/json to reject YAML superset syntax")
		}
		for _, c := range allCandidates() {
			if _, err := c.parse([]byte(superset)); err != nil {
				t.Errorf("expected %s to accept YAML superset syntax, got %v", c.name, err)
			}
		}
	})
}

// -----------------------------------------------------------------------------
// Criterion 4: YAML 1.1 vs 1.2 quirks in the Node API
// -----------------------------------------------------------------------------

func TestCriterion4_YAML11Vs12Quirks(t *testing.T) {
	input := strings.Join([]string{
		`k_yes: yes`,
		`k_no: no`,
		`k_on: on`,
		`k_off: off`,
		`k_y: y`,
		`k_n: n`,
		`k_oct_11: 0777`,
		`k_oct_12: 0o777`,
		`k_underscores: 1_000`,
		`k_tilde: ~`,
		`k_inf: .inf`,
		`k_date: 2026-11-07`,
	}, "\n") + "\n"

	type wantQuirk struct {
		kind   Kind
		scalar string
		tagV4  string
		tagGoc string
	}

	wants := []struct {
		key  string
		want wantQuirk
	}{
		{"k_yes", wantQuirk{KindScalar, "yes", "!!str", "!!str"}},
		{"k_no", wantQuirk{KindScalar, "no", "!!str", "!!str"}},
		{"k_on", wantQuirk{KindScalar, "on", "!!str", "!!str"}},
		{"k_off", wantQuirk{KindScalar, "off", "!!str", "!!str"}},
		{"k_y", wantQuirk{KindScalar, "y", "!!str", "!!str"}},
		{"k_n", wantQuirk{KindScalar, "n", "!!str", "!!str"}},
		{"k_oct_11", wantQuirk{KindScalar, "0777", "!!int", "!!int"}},
		{"k_oct_12", wantQuirk{KindScalar, "0o777", "!!int", "!!int"}},
		{"k_underscores", wantQuirk{KindScalar, "1_000", "!!int", "!!int"}},
		{"k_tilde", wantQuirk{KindNull, "~", "!!null", "!!null"}},
		{"k_inf", wantQuirk{KindScalar, ".inf", "!!float", "!!float"}},
		{"k_date", wantQuirk{KindScalar, "2026-11-07", "!!timestamp", "!!str"}},
	}

	for _, c := range allCandidates() {
		t.Run(c.name, func(t *testing.T) {
			root, err := c.parse([]byte(input))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			for _, tc := range wants {
				node, ok := root.Lookup(tc.key)
				if !ok {
					t.Fatalf("missing key %s", tc.key)
				}
				wantTag := tc.want.tagV4
				if c.name == "github.com/goccy/go-yaml" {
					wantTag = tc.want.tagGoc
				}
				if node.Kind != tc.want.kind || node.Scalar != tc.want.scalar || node.Tag != wantTag || node.Quoted {
					t.Errorf("%s: got {Kind:%v Scalar:%q Tag:%s Quoted:%v}, want {Kind:%v Scalar:%q Tag:%s Quoted:false}",
						tc.key, node.Kind, node.Scalar, node.Tag, node.Quoted, tc.want.kind, tc.want.scalar, wantTag)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Criterion 5: Safety (aliases / billion laughs, merge keys, multi-doc, deep nesting)
// -----------------------------------------------------------------------------

func makeBillionLaughsYAML(levels int) []byte {
	var sb strings.Builder
	sb.WriteString("a0: &a0 [\"lol\", \"lol\", \"lol\", \"lol\", \"lol\", \"lol\", \"lol\", \"lol\", \"lol\"]\n")
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&sb, "a%d: &a%d [*a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d]\n",
			i, i, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1)
	}
	return []byte(sb.String())
}

func TestCriterion5_Safety(t *testing.T) {
	t.Run("BillionLaughsAliasExpansion", func(t *testing.T) {
		bomb := makeBillionLaughsYAML(8) // 9^9 = 387,420,489 leaf strings if expanded

		// 1. In go.yaml.in/yaml/v4, go.yaml.in/yaml/v3, and gopkg.in/yaml.v3,
		// decoding into Go values enforces built-in alias expansion ratio limits:
		var anyV4 any
		if err := yamlv4.Load(bomb, &anyV4); err == nil || !strings.Contains(err.Error(), "excessive aliasing") {
			t.Fatalf("expected yaml/v4 Load to reject billion laughs, got: %v", err)
		}
		var anyV3 any
		if err := yamlv3.Unmarshal(bomb, &anyV3); err == nil || !strings.Contains(err.Error(), "excessive aliasing") {
			t.Fatalf("expected yaml/v3 Unmarshal to reject billion laughs, got: %v", err)
		}
		var anyGopkg any
		if err := gopkgv3.Unmarshal(bomb, &anyGopkg); err == nil || !strings.Contains(err.Error(), "excessive aliasing") {
			t.Fatalf("expected gopkg.in/yaml.v3 Unmarshal to reject billion laughs, got: %v", err)
		}

		// 2. In our Node builders, aliases are either rejected outright (default)
		// or bounded by maxExpansions:
		for _, c := range allCandidates() {
			if _, err := c.parse(bomb); err == nil {
				t.Errorf("expected %s Node builder to reject aliases, got nil", c.name)
			}
		}
		if _, err := ParseV4AllowAliases(bomb, 100); err == nil || !strings.Contains(err.Error(), "alias expansion limit (100) exceeded") {
			t.Errorf("expected ParseV4AllowAliases to enforce expansion cap, got %v", err)
		}

		// 3. Demonstrate goccy/go-yaml shortcoming: goccy has NO alias expansion
		// counter when decoding into typed Go slices/structs (where decodeSlice
		// allocates a new slice per alias reference). Even a 5-level bomb (9^6 = 531,441
		// strings) expands all 531,441 elements in memory without any limit!
		smallBomb := makeBillionLaughsYAML(5)
		var goccyTyped map[string]any
		// When decoding into typed slices, e.g. 5 levels:
		type bombTarget struct {
			A0 []string           `yaml:"a0"`
			A1 [][]string         `yaml:"a1"`
			A2 [][][]string       `yaml:"a2"`
			A3 [][][][]string     `yaml:"a3"`
			A4 [][][][][]string   `yaml:"a4"`
			A5 [][][][][][]string `yaml:"a5"`
		}
		var bt bombTarget
		if err := goccy.Unmarshal(smallBomb, &bt); err != nil {
			t.Fatalf("goccy unmarshal failed: %v", err)
		}
		_ = goccyTyped
		// Count leaf strings in bt.A5: 9^6 = 531,441 allocated strings with no limit!
		leafCount := len(bt.A5) * len(bt.A5[0]) * len(bt.A5[0][0]) * len(bt.A5[0][0][0]) * len(bt.A5[0][0][0][0]) * len(bt.A5[0][0][0][0][0])
		if leafCount != 531441 {
			t.Errorf("goccy expanded %d leaves, want 531441", leafCount)
		}
	})

	t.Run("MergeKeys", func(t *testing.T) {
		mergeYAML := "base: &b\n  x: 1\nchild:\n  <<: *b\n  y: 2\n"
		for _, c := range allCandidates() {
			_, err := c.parse([]byte(mergeYAML))
			if err == nil || !strings.Contains(err.Error(), "merge key '<<' is not supported") {
				t.Errorf("%s: expected merge key rejection with position, got %v", c.name, err)
			}
		}

		// Verify quoted "<<" is treated as a normal literal key, not a merge key:
		quotedMerge := "\"<<\": literal\n"
		for _, c := range allCandidates() {
			root, err := c.parse([]byte(quotedMerge))
			if err != nil {
				t.Fatalf("%s rejected quoted '<<': %v", c.name, err)
			}
			if n, ok := root.Lookup("<<"); !ok || n.Scalar != "literal" {
				t.Errorf("%s Lookup(\"<<\") = %+v", c.name, n)
			}
		}
	})

	t.Run("MultiDocumentStreams", func(t *testing.T) {
		multiDoc := "a: 1\n---\nb: 2\n"

		// go.yaml.in/yaml/v4's yaml.Load rejects multi-document streams by default!
		var docV4 yamlv4.Node
		errV4 := yamlv4.Load([]byte(multiDoc), &docV4)
		if errV4 == nil || !strings.Contains(errV4.Error(), "expected single document, found multiple") {
			t.Fatalf("expected yamlv4.Load to reject multi-doc stream by default, got %v", errV4)
		}

		// By contrast, yamlv3.Unmarshal, gopkgv3.Unmarshal, and goccy.Unmarshal silently ignore doc 2:
		var docV3 yamlv3.Node
		if err := yamlv3.Unmarshal([]byte(multiDoc), &docV3); err != nil {
			t.Fatalf("expected yamlv3.Unmarshal to silently ignore second doc, got %v", err)
		}
		var docGopkg gopkgv3.Node
		if err := gopkgv3.Unmarshal([]byte(multiDoc), &docGopkg); err != nil {
			t.Fatalf("expected gopkgv3.Unmarshal to silently ignore second doc, got %v", err)
		}
		var goccyMap map[string]any
		if err := goccy.Unmarshal([]byte(multiDoc), &goccyMap); err != nil {
			t.Fatalf("expected goccy.Unmarshal to silently ignore second doc, got %v", err)
		}

		// All 4 of our wrapper builders reject multi-document streams:
		for _, c := range allCandidates() {
			if _, err := c.parse([]byte(multiDoc)); err == nil {
				t.Errorf("expected %s wrapper to reject multi-document stream", c.name)
			}
		}
	})

	t.Run("VeryDeepNesting", func(t *testing.T) {
		const depth = 10005
		deepFlow := bytes.Repeat([]byte("["), depth)
		deepFlow = append(deepFlow, bytes.Repeat([]byte("]"), depth)...)

		// yaml/v4, yaml/v3, and gopkg.in/yaml.v3 enforce max_nesting_level = 10000 in the scanner/parser:
		for _, c := range []candidateParser{
			{name: "go.yaml.in/yaml/v4", parse: ParseV4},
			{name: "go.yaml.in/yaml/v3", parse: ParseV3},
			{name: "gopkg.in/yaml.v3", parse: ParseGopkgV3},
		} {
			_, err := c.parse(deepFlow)
			if err == nil || !strings.Contains(err.Error(), "exceeded max depth of 10000") {
				t.Errorf("%s: expected max depth error, got %v", c.name, err)
			}
		}

		// goccy/go-yaml's parser.ParseBytes has NO depth limit at all (its maxDecodeDepth=10000
		// check only runs in Decoder.decodeNode after parser.ParseBytes completes!).
		// We verify with depth=500 that parser.ParseBytes builds an arbitrarily deep AST.
		const modDepth = 500
		modFlow := append(bytes.Repeat([]byte("["), modDepth), bytes.Repeat([]byte("]"), modDepth)...)
		f, err := goccyparser.ParseBytes(modFlow, 0)
		if err != nil {
			t.Fatalf("goccy ParseBytes failed on depth %d: %v", modDepth, err)
		}
		cur := f.Docs[0].Body
		actualDepth := 0
		for {
			seq, ok := cur.(*goccyast.SequenceNode)
			if !ok {
				break
			}
			actualDepth++
			if len(seq.Values) == 0 {
				break
			}
			cur = seq.Values[0]
		}
		if actualDepth != modDepth {
			t.Errorf("goccy AST depth = %d, want %d", actualDepth, modDepth)
		}
	})
}

// -----------------------------------------------------------------------------
// Criterion 6: Error messages (syntax and structure errors with line:column)
// -----------------------------------------------------------------------------

func TestCriterion6_ErrorMessages(t *testing.T) {
	syntaxErrYAML := "a: [1, 2\nb: 3\n"
	structureErrYAML := "a:\n  b: 1\n c: 2\n"
	line1ScannerErr := "{\"a\": \"\\uD800\"}\n"

	v4LineColRe := regexp.MustCompile(`L(\d+)\.C(\d+)`)
	goccyLineColRe := regexp.MustCompile(`\[(\d+):(\d+)\]`)
	v3LineOnlyRe := regexp.MustCompile(`line (\d+):`)

	for _, input := range []struct {
		name string
		data string
	}{
		{"syntax_flow_seq", syntaxErrYAML},
		{"structure_indent", structureErrYAML},
		{"line1_scanner_escape", line1ScannerErr},
	} {
		t.Run(input.name, func(t *testing.T) {
			// 1. go.yaml.in/yaml/v4 includes L<line>.C<col> in error string AND structured Mark{Line, Column}
			_, errV4 := ParseV4([]byte(input.data))
			if errV4 == nil {
				t.Fatalf("expected v4 error")
			}
			if !v4LineColRe.MatchString(errV4.Error()) {
				t.Errorf("v4 error %q missing L<line>.C<col>", errV4.Error())
			}
			var loadErr *yamlv4.LoadError
			if !errors.As(errV4, &loadErr) || loadErr.Mark.Line <= 0 || loadErr.Mark.Column <= 0 {
				t.Errorf("v4 error does not unwrap to *yamlv4.LoadError with positive Line/Column: %T %+v", errV4, loadErr)
			}

			// 2. go.yaml.in/yaml/v3 and gopkg.in/yaml.v3 only include `line N` (NO column),
			// and on line-1 scanner errors omit even the line number!
			_, errV3 := ParseV3([]byte(input.data))
			if errV3 == nil {
				t.Fatalf("expected v3 error")
			}
			if input.name == "line1_scanner_escape" {
				if v3LineOnlyRe.MatchString(errV3.Error()) {
					t.Errorf("unexpected line number in v3 line-1 scanner error: %q", errV3.Error())
				}
			} else {
				if !v3LineOnlyRe.MatchString(errV3.Error()) {
					t.Errorf("v3 error %q missing 'line N:'", errV3.Error())
				}
			}
			if strings.Contains(strings.ToLower(errV3.Error()), "col") {
				t.Errorf("unexpected column info in v3 error: %q", errV3.Error())
			}

			// 3. github.com/goccy/go-yaml includes [line:col] on syntax/structure errors
			// (Note: goccy does not error on \uD800 because it decodes lone surrogates to U+FFFD).
			if input.name != "line1_scanner_escape" {
				_, errGoccy := ParseGoccy([]byte(input.data))
				if errGoccy == nil {
					t.Fatalf("expected goccy error")
				}
				if !goccyLineColRe.MatchString(errGoccy.Error()) {
					t.Errorf("goccy error %q missing [line:col]", errGoccy.Error())
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Criterion 8: Dependency weight and binary-size increase
// -----------------------------------------------------------------------------

func TestCriterion8_BinarySizeAndDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build size test in -short mode")
	}

	programs := []struct {
		name string
		src  string
	}{
		{
			name: "baseline_none",
			src: `package main
import (
	"fmt"
	"os"
)
func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil { panic(err) }
	fmt.Println(len(b))
}
`,
		},
		{
			name: "go.yaml.in/yaml/v4",
			src: `package main
import (
	"fmt"
	"os"
	yaml "go.yaml.in/yaml/v4"
)
func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil { panic(err) }
	var n yaml.Node
	if err := yaml.Load(b, &n); err != nil { panic(err) }
	fmt.Println(n.Kind, n.Line, n.Column, len(n.Content))
}
`,
		},
		{
			name: "go.yaml.in/yaml/v3",
			src: `package main
import (
	"fmt"
	"os"
	yaml "go.yaml.in/yaml/v3"
)
func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil { panic(err) }
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil { panic(err) }
	fmt.Println(n.Kind, n.Line, n.Column, len(n.Content))
}
`,
		},
		{
			name: "gopkg.in/yaml.v3",
			src: `package main
import (
	"fmt"
	"os"
	yaml "gopkg.in/yaml.v3"
)
func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil { panic(err) }
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil { panic(err) }
	fmt.Println(n.Kind, n.Line, n.Column, len(n.Content))
}
`,
		},
		{
			name: "github.com/goccy/go-yaml",
			src: `package main
import (
	"fmt"
	"os"
	"github.com/goccy/go-yaml/parser"
)
func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil { panic(err) }
	f, err := parser.ParseBytes(b, 0)
	if err != nil { panic(err) }
	fmt.Println(len(f.Docs))
}
`,
		},
	}

	tmpDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	sizes := make(map[string]int64, len(programs))
	for i, p := range programs {
		srcDir := filepath.Join(tmpDir, fmt.Sprintf("prog_%d", i))
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		mainFile := filepath.Join(srcDir, "main.go")
		if err := os.WriteFile(mainFile, []byte(p.src), 0o644); err != nil {
			t.Fatal(err)
		}
		binFile := filepath.Join(srcDir, "prog")
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", binFile, mainFile)
		cmd.Dir = wd
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build %s failed: %v\n%s", p.name, err, out)
		}
		info, err := os.Stat(binFile)
		if err != nil {
			t.Fatal(err)
		}
		sizes[p.name] = info.Size()
	}

	base := sizes["baseline_none"]
	t.Logf("Baseline binary size (no YAML library): %d bytes (%.1f KiB)", base, float64(base)/1024.0)
	for _, p := range programs[1:] {
		sz := sizes[p.name]
		delta := sz - base
		t.Logf("%-25s binary=%d bytes (%.1f KiB) delta=+%d bytes (+%.1f KiB)",
			p.name, sz, float64(sz)/1024.0, delta, float64(delta)/1024.0)
		if delta <= 0 {
			t.Errorf("expected positive binary size delta for %s, got %d", p.name, delta)
		}
	}
}

// -----------------------------------------------------------------------------
// Criterion 9: Speed benchmark (~100-key config as YAML and JSON into Node walk)
// -----------------------------------------------------------------------------

func generate100KeyConfigs() (yamlBytes, jsonBytes []byte) {
	var ysb strings.Builder
	rootObj := make(map[string]any, 10)

	// 10 sections * 10 leaf keys each = 100 keys
	for s := 0; s < 10; s++ {
		secName := fmt.Sprintf("section_%02d", s)
		fmt.Fprintf(&ysb, "%s:\n", secName)
		secMap := make(map[string]any, 10)

		fmt.Fprintf(&ysb, "  host: \"svc-%02d.internal.example.com\"\n", s)
		secMap["host"] = fmt.Sprintf("svc-%02d.internal.example.com", s)

		fmt.Fprintf(&ysb, "  port: %d\n", 8000+s)
		secMap["port"] = 8000 + s

		fmt.Fprintf(&ysb, "  enabled: %v\n", s%2 == 0)
		secMap["enabled"] = s%2 == 0

		fmt.Fprintf(&ysb, "  timeout: \"15s\"\n")
		secMap["timeout"] = "15s"

		fmt.Fprintf(&ysb, "  rate_limit: %.2f\n", 125.5+float64(s))
		secMap["rate_limit"] = 125.5 + float64(s)

		fmt.Fprintf(&ysb, "  max_conns: %d\n", 64*(s+1))
		secMap["max_conns"] = 64 * (s + 1)

		fmt.Fprintf(&ysb, "  region: \"eu-central-%d\"\n", (s%3)+1)
		secMap["region"] = fmt.Sprintf("eu-central-%d", (s%3)+1)

		fmt.Fprintf(&ysb, "  retry_backoff: \"250ms\"\n")
		secMap["retry_backoff"] = "250ms"

		fmt.Fprintf(&ysb, "  tags:\n    - \"prod\"\n    - \"tier-%d\"\n    - \"blue\"\n", s%4)
		secMap["tags"] = []string{"prod", fmt.Sprintf("tier-%d", s%4), "blue"}

		fmt.Fprintf(&ysb, "  tls:\n    verify: true\n    min_version: \"1.3\"\n")
		secMap["tls"] = map[string]any{"verify": true, "min_version": "1.3"}

		rootObj[secName] = secMap
	}

	jBytes, err := json.MarshalIndent(rootObj, "", "  ")
	if err != nil {
		panic(err)
	}
	return []byte(ysb.String()), jBytes
}

func BenchmarkCriterion9_Parse100Keys(b *testing.B) {
	yamlData, jsonData := generate100KeyConfigs()

	for _, c := range allCandidates() {
		b.Run(c.name+"/YAML", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(yamlData)))
			for b.Loop() {
				n, err := c.parse(yamlData)
				if err != nil || n == nil {
					b.Fatalf("parse failed: %v", err)
				}
			}
		})
		b.Run(c.name+"/JSON", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(jsonData)))
			for b.Loop() {
				n, err := c.parse(jsonData)
				if err != nil || n == nil {
					b.Fatalf("parse failed: %v", err)
				}
			}
		})
	}

	b.Run("encoding_json_reference/JSON", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(jsonData)))
		for b.Loop() {
			var out map[string]any
			if err := json.Unmarshal(jsonData, &out); err != nil {
				b.Fatalf("json.Unmarshal failed: %v", err)
			}
		}
	})
}
