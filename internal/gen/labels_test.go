package gen

import (
	"bytes"
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/epistemer/cfgx/dsl"
	"github.com/epistemer/cfgx/internal/gen/testdata/labeled"
	"github.com/epistemer/cfgx/internal/golden"
	"github.com/epistemer/cfgx/internal/ir"
)

func labelsApp() (*Model, error) {
	return Build(labelsIR(), "settings")
}

// TestLabelsGolden is the labels file of a schema with nested groups and an
// enum whose description needs quoting. Command Args get no labels.
func TestLabelsGolden(t *testing.T) {
	m, err := labelsApp()
	if err != nil {
		t.Fatal(err)
	}
	src, err := RenderLabels(m, "internal/settings")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, filepath.Join("testdata", "labels.golden.json"), src)
	if strings.Contains(string(src), "to.address") || strings.Contains(string(src), "migrate") {
		t.Error("arguments are flags of one run and get no labels")
	}
	if !strings.Contains(string(src), `"settings.scheduler.execmode"`) {
		t.Error("labeled enum key must keep its own label")
	}
	if strings.Contains(string(src), "settings.scheduler.execmode.") {
		t.Error("labeled enum must get no value labels")
	}
	if !strings.Contains(string(src), `"settings.database.sslmode.disable"`) {
		t.Error("plain enum must keep its value labels")
	}

	m, err = Build(nested(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	if m.Labels || len(m.L10n) > 0 {
		t.Error("labels without App.Labels")
	}
}

func checkLabelsDoc(t *testing.T, src []byte) {
	t.Helper()
	var doc, schema any
	if err := json.Unmarshal(src, &doc); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("testdata", "labels.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	for _, p := range validate(schema.(map[string]any), doc, "") {
		t.Error(p)
	}
	seen := map[string]bool{}
	for _, l := range doc.(map[string]any)["labels"].([]any) {
		l := l.(map[string]any)
		for _, f := range []string{"var", "path"} {
			v, ok := l[f].(string)
			if !ok {
				continue
			}
			if seen[f+v] {
				t.Errorf("%s %q twice", f, v)
			}
			seen[f+v] = true
			if f == "var" && (!token.IsIdentifier(v) || v == "L10n" || v == "l10n") {
				t.Errorf("var %q isn't a usable identifier", v)
			}
		}
	}
}

// TestLabelsSchema checks the file against genx's labels/v1 JSON Schema,
// copied into testdata, and the rules genx adds to it (interface §11).
func TestLabelsSchema(t *testing.T) {
	m, err := labelsApp()
	if err != nil {
		t.Fatal(err)
	}
	src, err := RenderLabels(m, "internal/settings")
	if err != nil {
		t.Fatal(err)
	}
	checkLabelsDoc(t, src)
}

var (
	dslLabelsOnce sync.Once
	dslLabelsIR   []byte
	dslLabelsErr  error
)

// TestLabelsEnumLabeledFromDSL runs a real DSL schema importing a named enum
// type with Label() string alongside a plain string enum through Export, Build
// and RenderLabels, verifying the schema and labels output end to end.
func TestLabelsEnumLabeledFromDSL(t *testing.T) {
	dslLabelsOnce.Do(func() {
		g := dsl.Group("log", "Logging",
			dsl.Enum("mode", "Execution mode", labeled.Sequential, labeled.Parallel).Default(labeled.Sequential),
			dsl.Enum("level", "Log level", "debug", "info").Default("info"),
		)
		dsl.App("app", "Test app", g).Labels()
		dsl.Command("Run").Uses(g)
		var buf bytes.Buffer
		dslLabelsErr = dsl.Export(&buf)
		dslLabelsIR = buf.Bytes()
	})
	if dslLabelsErr != nil {
		t.Fatal(dslLabelsErr)
	}
	app, err := ir.Decode(bytes.NewReader(dslLabelsIR))
	if err != nil {
		t.Fatal(err)
	}
	m, err := Build(app, "settings")
	if err != nil {
		t.Fatal(err)
	}
	src, err := RenderLabels(m, "internal/settings")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, filepath.Join("testdata", "labels_enum_dsl.golden.json"), src)
	checkLabelsDoc(t, src)
	s := string(src)
	if !strings.Contains(s, `"settings.log.mode"`) {
		t.Error("labeled enum key missing its own label")
	}
	if strings.Contains(s, "settings.log.mode.") {
		t.Error("labeled enum must not emit value labels")
	}
	if !strings.Contains(s, `"settings.log.level.debug"`) || !strings.Contains(s, `"settings.log.level.info"`) {
		t.Error("plain enum must emit value labels")
	}
}

// validate is enough of JSON Schema for labels.v1.schema.json: type,
// required, properties, items, const, minLength and pattern.
func validate(s map[string]any, v any, at string) []string {
	var problems []string
	bad := func(msg string) { problems = append(problems, at+": "+msg) }
	if c, ok := s["const"]; ok && c != v {
		bad("want " + c.(string))
	}
	switch s["type"] {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			bad("want an object")
			return problems
		}
		for _, r := range asList(s["required"]) {
			if _, ok := obj[r.(string)]; !ok {
				bad(r.(string) + " is required")
			}
		}
		props, _ := s["properties"].(map[string]any)
		for k, sub := range props {
			if x, ok := obj[k]; ok {
				problems = append(problems, validate(sub.(map[string]any), x, at+"."+k)...)
			}
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			bad("want an array")
			return problems
		}
		for i, x := range arr {
			problems = append(problems, validate(s["items"].(map[string]any), x, at+"["+strconv.Itoa(i)+"]")...)
		}
	case "string":
		str, ok := v.(string)
		if !ok {
			bad("want a string")
			return problems
		}
		if n, ok := s["minLength"].(float64); ok && len(str) < int(n) {
			bad("too short")
		}
		if p, ok := s["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(str) {
			bad("doesn't match " + p)
		}
	}
	return problems
}

func asList(v any) []any { l, _ := v.([]any); return l }
