package gen

import (
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/golden"
)

func labelsApp() (*Model, error) {
	a := i18nApp()
	a.I18n = false
	a.Labels = true
	return Build(a, "settings")
}

// TestLabelsGolden is M11.1's labels file for the schema TestI18nGolden uses.
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

	m, err = Build(nested(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	if m.Labels || len(m.L10n) > 0 {
		t.Error("labels without App.Labels")
	}
}

// TestLabelsMatchI18n: the labels file holds the records RenderI18n prints,
// in the same order, so i18n's generator can write the same declarations.
func TestLabelsMatchI18n(t *testing.T) {
	m, err := labelsApp()
	if err != nil {
		t.Fatal(err)
	}
	src, err := RenderLabels(m, "internal/settings")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Labels []struct{ Key, Text, Path, Var string }
	}
	if err := json.Unmarshal(src, &doc); err != nil {
		t.Fatal(err)
	}
	mi, err := Build(i18nApp(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	goSrc, err := RenderI18n(mi)
	if err != nil {
		t.Fatal(err)
	}
	// Each entry of RenderI18n's l10n table: "path": Var or i18n.Key(key, text).
	entry := regexp.MustCompile(`(?m)^\t("[^"]*"):\s+(L10n\w+|i18n\.Key\(("[^"]*"), ("(?:[^"\\]|\\.)*")\)),$`)
	vars := regexp.MustCompile(`(?m)^\t(L10n\w+) += i18n\.Key\(("[^"]*"), ("(?:[^"\\]|\\.)*")\)$`)
	decl := map[string][2]string{}
	for _, v := range vars.FindAllStringSubmatch(string(goSrc), -1) {
		decl[v[1]] = [2]string{v[2], v[3]}
	}
	got := entry.FindAllStringSubmatch(string(goSrc), -1)
	if len(got) != len(doc.Labels) || len(got) == 0 {
		t.Fatalf("RenderI18n has %d labels, the labels file %d", len(got), len(doc.Labels))
	}
	for i, l := range doc.Labels {
		g := got[i]
		key, text := g[3], g[4]
		if strings.HasPrefix(g[2], "L10n") {
			if g[2] != l.Var {
				t.Errorf("label %d: var %s, RenderI18n has %s", i, l.Var, g[2])
			}
			key, text = decl[g[2]][0], decl[g[2]][1]
		} else if l.Var != "" {
			t.Errorf("label %d: var %s, RenderI18n has none", i, l.Var)
		}
		if want := quote(l.Path); g[1] != want {
			t.Errorf("label %d: path %s, RenderI18n has %s", i, want, g[1])
		}
		if quote(l.Key) != key || quote(l.Text) != text {
			t.Errorf("label %d: %q %q, RenderI18n has %s %s", i, l.Key, l.Text, key, text)
		}
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return strings.ReplaceAll(string(b), `&`, "&")
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

func TestLabelsAndI18nRefused(t *testing.T) {
	a := i18nApp()
	a.Labels = true
	if _, err := Build(a, "settings"); err == nil || !strings.Contains(err.Error(), "App.I18n and App.Labels both declare") {
		t.Errorf("both: %v", err)
	}
}
