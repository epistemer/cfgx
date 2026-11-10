package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/epistemer/cfgx/engine"
	"github.com/epistemer/cfgx/internal/ir"
)

// Artifacts renders the files cfgx writes next to the generated code (M6.1),
// by path relative to the output directory: the example config and its
// JSON Schema (when the app has a config file), .env.example, docs/CONFIG.md
// and cfgx.lock.json. It also sets m.ExampleConfig and m.JSONSchema, which
// the generated file embeds, so call it before Render. Output is
// deterministic.
func Artifacts(m *Model) (map[string][]byte, error) {
	a := &artifacts{m: m, usedBy: usedBy(m)}
	out := map[string][]byte{}
	if m.ConfigFile != "" {
		y := a.exampleYAML()
		s, err := a.jsonSchema()
		if err != nil {
			return nil, err
		}
		out[m.ConfigFile+".example.yaml"] = y
		out[m.ConfigFile+".schema.json"] = s
		m.ExampleConfig, m.JSONSchema = string(y), string(s)
	}
	if env := a.envExample(); env != nil {
		out[".env.example"] = env
	}
	out["docs/CONFIG.md"] = a.configMD()
	lock, err := Lock(m.app)
	if err != nil {
		return nil, err
	}
	out["cfgx.lock.json"] = lock
	return out, nil
}

type artifacts struct {
	m      *Model
	usedBy map[int]string
}

// usedBy says, for each key index, which commands load it: "every command"
// for a global key, else their names, else "no command".
func usedBy(m *Model) map[int]string {
	names := map[int][]string{}
	for _, c := range m.Commands {
		for _, i := range c.KeyIdx {
			names[i] = append(names[i], c.Name)
		}
	}
	out := map[int]string{}
	for _, k := range m.Keys {
		switch {
		case k.Global:
			out[k.Index] = "every command"
		case len(names[k.Index]) == 0:
			out[k.Index] = "no command"
		default:
			out[k.Index] = strings.Join(names[k.Index], ", ")
		}
	}
	return out
}

// inFile reports whether the key can be set in the config file.
func inFile(k *KeyModel) bool { return !k.FlagOnly && !k.NoFile }

// value is the key's example or default as a JSON value, or nil if there is
// none to show: no default, a computed one, or a secret's.
func value(k *KeyModel) any {
	if k.ir.Kind == ir.KindSecret || k.defaultFunc != nil {
		return nil
	}
	raw, canonical := "", ""
	switch {
	case k.ir.Example != nil:
		raw, canonical = *k.ir.Example, k.Example
	case k.HasDefault && k.ir.Default != nil:
		raw, canonical = *k.ir.Default, k.DefaultText
	default:
		return nil
	}
	if raw == "" {
		return nil
	}
	switch k.ir.Kind {
	case ir.KindBool:
		b, err := strconv.ParseBool(canonical)
		if err != nil {
			return canonical
		}
		return b
	case ir.KindInt, ir.KindInt64, ir.KindUint, ir.KindFloat:
		return json.Number(canonical)
	case ir.KindStrings, ir.KindInts, ir.KindDurations, ir.KindStringMap:
		items, err := engine.SplitCSV(raw)
		if err != nil {
			return nil
		}
		switch k.ir.Kind {
		case ir.KindInts:
			ns := make([]json.Number, len(items))
			for i, it := range items {
				ns[i] = json.Number(strings.TrimSpace(it))
			}
			return ns
		case ir.KindDurations:
			for i, it := range items {
				if d, err := time.ParseDuration(strings.TrimSpace(it)); err == nil {
					items[i] = d.String()
				}
			}
		case ir.KindStringMap:
			mp := map[string]string{}
			for _, it := range items {
				key, v, _ := strings.Cut(it, "=")
				mp[strings.TrimSpace(key)] = v
			}
			return mp
		}
		return items
	}
	return canonical
}

// envValue is the key's example or default as an env var's text.
func envValue(k *KeyModel) string {
	switch v := value(k).(type) {
	case nil:
		return ""
	case []string, []json.Number, map[string]string:
		// Lists and maps take the schema's CSV text.
		if k.ir.Example != nil {
			return *k.ir.Example
		}
		return *k.ir.Default
	default:
		return fmt.Sprint(v)
	}
}

func jsonText(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// fileNode is one level of the config file: a group, a segment of a dotted key
// name, or a key.
type fileNode struct {
	name, desc string
	kids       []*fileNode
	key        *KeyModel
}

func (n *fileNode) child(name string) *fileNode {
	for _, k := range n.kids {
		if k.name == name && k.key == nil {
			return k
		}
	}
	c := &fileNode{name: name}
	n.kids = append(n.kids, c)
	return c
}

// fileTree arranges the keys that keep returns true for by their config
// file paths, with group descriptions.
func (a *artifacts) fileTree(keep func(*KeyModel) bool) *fileNode {
	descs := map[string]string{}
	for _, g := range a.m.Groups {
		descs[g.Path] = g.Desc
	}
	root := &fileNode{}
	for _, k := range a.m.Keys {
		if !inFile(k) || !keep(k) {
			continue
		}
		segs := strings.Split(k.Path, ".")
		n := root
		for i, s := range segs[:len(segs)-1] {
			n = n.child(s)
			n.desc = descs[strings.Join(segs[:i+1], ".")]
		}
		n.kids = append(n.kids, &fileNode{name: segs[len(segs)-1], key: k})
	}
	return root
}

// keyNote is what the example files say about a key, one sentence each.
// In .env.example (env true) it names the config key instead of the env
// var.
func (a *artifacts) keyNote(k *KeyModel, env bool) []string {
	desc := strings.TrimSuffix(strings.TrimSpace(k.Desc), ".")
	if len(k.Enum) > 0 {
		texts := make([]string, len(k.Enum))
		for i, e := range k.Enum {
			texts[i] = e.Text
		}
		desc += "; one of " + strings.Join(texts, ", ")
	}
	lines := []string{desc + "."}
	var notes []string
	if k.Required {
		notes = append(notes, "Required.")
	}
	if k.ir.Kind == ir.KindSecret {
		notes = append(notes, "Secret: set it in the environment, not here.")
	}
	if k.DefaultFrom != "" {
		notes = append(notes, "Default: from "+k.DefaultFrom+".")
	}
	if k.ir.Example != nil && k.HasDefault && k.DefaultText != "" {
		notes = append(notes, "Default: "+k.DefaultText+"; the value shown is an example.")
	}
	if k.Deprecated != "" {
		notes = append(notes, "Deprecated: "+strings.TrimSuffix(k.Deprecated, ".")+".")
	}
	if len(notes) > 0 {
		lines = append(lines, strings.Join(notes, " "))
	}
	var where []string
	if env && inFile(k) {
		where = append(where, "key "+k.Path)
	}
	if k.Env != "" && !env {
		e := "env " + k.Env
		if k.FileEnv {
			e += " or " + k.Env + "_FILE"
		}
		where = append(where, e)
	}
	if k.Flag != "" {
		where = append(where, "flag --"+k.Flag)
	}
	w := "Used by " + a.usedBy[k.Index] + "."
	if len(where) > 0 {
		w = strings.Join(where, ", ") + ". " + w
		w = strings.ToUpper(w[:1]) + w[1:]
	}
	return append(lines, w)
}

// exampleYAML is the example config file. Every key is commented out at its
// example or default, so a copy of it changes nothing until a line is
// uncommented, and later defaults still apply.
func (a *artifacts) exampleYAML() []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s.\n", a.m.AppName, strings.TrimSuffix(a.m.AppDesc, "."))
	b.WriteString("# Generated by cfgx from the schema; don't edit. Every setting is commented\n")
	b.WriteString("# out at its default: uncomment a line to change it.\n")
	root := a.fileTree(func(k *KeyModel) bool { return !k.Hidden })
	var walk func(n *fileNode, indent string)
	walk = func(n *fileNode, indent string) {
		for i, c := range n.kids {
			if c.key == nil {
				b.WriteString("\n")
				if c.desc != "" {
					comment(&b, indent, c.desc)
				}
				fmt.Fprintf(&b, "%s%s:\n", indent, c.name)
				walk(c, indent+"  ")
				continue
			}
			if i > 0 && n.kids[i-1].key != nil {
				b.WriteString("\n")
			}
			for _, l := range a.keyNote(c.key, false) {
				comment(&b, indent, l)
			}
			if v := value(c.key); v != nil {
				fmt.Fprintf(&b, "%s# %s: %s\n", indent, c.name, jsonText(v))
			} else {
				fmt.Fprintf(&b, "%s# %s:\n", indent, c.name)
			}
		}
	}
	walk(root, "")
	return []byte(b.String())
}

// envExample lists the env vars, commented out at their defaults; nil if
// no key has one.
func (a *artifacts) envExample() []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: environment variables.\n", a.m.AppName)
	b.WriteString("# Generated by cfgx from the schema; don't edit. Each is commented out at\n")
	b.WriteString("# its default.\n")
	if a.m.ConfigFile != "" {
		fmt.Fprintf(&b, "\n# The config file to read, instead of searching for %s.yaml.\n# %s_CONFIG=\n", a.m.ConfigFile, a.m.EnvPrefix)
	}
	n := 0
	for _, k := range a.m.Keys {
		if k.Env == "" || k.Hidden {
			continue
		}
		n++
		b.WriteString("\n")
		for _, l := range a.keyNote(k, true) {
			comment(&b, "", l)
		}
		fmt.Fprintf(&b, "# %s=%s\n", k.Env, dotenvQuote(envValue(k)))
	}
	if n == 0 && a.m.ConfigFile == "" {
		return nil
	}
	return []byte(b.String())
}

// dotenvQuote double-quotes a value that a .env reader would otherwise cut.
func dotenvQuote(s string) string {
	if s == "" || !strings.ContainsAny(s, " \t\r\n#\"'\\$`") {
		return s
	}
	return strconv.Quote(s)
}

const (
	durationPattern = `^([-+]?([0-9]+(\.[0-9]*)?|\.[0-9]+)(ns|us|µs|ms|s|m|h))+$|^0$`
	sizePattern     = `^\s*[0-9]+\s*([bB]|[kKmMgGtTpP][iI]?[bB])?\s*$`
)

// keySchema is a key's JSON Schema.
func keySchema(k *KeyModel) map[string]any {
	s := map[string]any{}
	str := func() { s["type"] = "string" }
	scalarItem := map[string]any{"type": []string{"string", "number", "boolean"}}
	switch k.ir.Kind {
	case ir.KindBool:
		s["type"] = "boolean"
	case ir.KindInt, ir.KindInt64:
		s["type"] = "integer"
	case ir.KindUint:
		s["type"], s["minimum"] = "integer", 0
	case ir.KindFloat:
		s["type"] = "number"
	case ir.KindDuration:
		s["anyOf"] = []any{map[string]any{"type": "string", "pattern": durationPattern}, map[string]any{"const": 0}}
	case ir.KindByteSize:
		s["anyOf"] = []any{map[string]any{"type": "integer", "minimum": 0}, map[string]any{"type": "string", "pattern": sizePattern}}
	case ir.KindEnum:
		texts := make([]string, len(k.Enum))
		for i, e := range k.Enum {
			texts[i] = e.Text
		}
		s["enum"] = texts
	case ir.KindStrings:
		s["anyOf"] = []any{map[string]any{"type": "array", "items": scalarItem}, map[string]any{"type": "string"}}
	case ir.KindInts:
		s["anyOf"] = []any{map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}, map[string]any{"type": "string"}}
	case ir.KindDurations:
		item := map[string]any{"anyOf": []any{map[string]any{"type": "string", "pattern": durationPattern}, map[string]any{"const": 0}}}
		s["anyOf"] = []any{map[string]any{"type": "array", "items": item}, map[string]any{"type": "string"}}
	case ir.KindStringMap:
		s["anyOf"] = []any{map[string]any{"type": "object", "additionalProperties": scalarItem}, map[string]any{"type": "string"}}
	case ir.KindURL:
		str()
		s["format"] = "uri"
	default:
		str()
	}
	for _, c := range k.ir.Checks {
		switch c.Kind {
		case ir.CheckMin, ir.CheckMax:
			if k.ir.Kind == ir.KindDuration || k.ir.Kind == ir.KindByteSize {
				continue
			}
			name := "minimum"
			if c.Kind == ir.CheckMax {
				name = "maximum"
			}
			s[name] = json.Number(c.Args[0])
		case ir.CheckLen:
			s["minLength"], s["maxLength"] = json.Number(c.Args[0]), json.Number(c.Args[1])
		case ir.CheckPattern:
			s["pattern"] = "^(?:" + c.Args[0] + ")$"
		}
	}
	// A null value counts as absent, as an uncommented "key:" is.
	switch {
	case s["type"] != nil:
		s["type"] = []any{s["type"], "null"}
	case s["anyOf"] != nil:
		s["anyOf"] = append(s["anyOf"].([]any), map[string]any{"type": "null"})
	case s["enum"] != nil:
		s["enum"] = append(toAny(s["enum"]), nil)
	}
	desc := k.Desc
	if k.Deprecated != "" {
		desc += " Deprecated: " + k.Deprecated
		s["deprecated"] = true
	}
	s["description"] = desc
	if v := value(k); v != nil && k.ir.Example == nil {
		s["default"] = v
	}
	if k.ir.Example != nil {
		if v := value(k); v != nil {
			s["examples"] = []any{v}
		}
	}
	return s
}

// jsonSchema is the config file's JSON Schema (draft 2020-12). Unknown keys
// are errors in an editor, as they are warnings in cfgx.
func (a *artifacts) jsonSchema() ([]byte, error) {
	root := a.fileTree(func(*KeyModel) bool { return true })
	var obj func(n *fileNode) map[string]any
	obj = func(n *fileNode) map[string]any {
		props := map[string]any{}
		for _, c := range n.kids {
			if c.key == nil {
				o := obj(c)
				if c.desc != "" {
					o["description"] = c.desc
				}
				props[c.name] = o
				continue
			}
			props[c.name] = keySchema(c.key)
		}
		return map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	}
	s := obj(root)
	// Old names still work, with a warning.
	for _, k := range a.m.Keys {
		if !inFile(k) {
			continue
		}
		for _, al := range k.Aliases {
			ks := keySchema(k)
			ks["deprecated"] = true
			ks["description"] = fmt.Sprintf("Deprecated: use %s. %s", k.Path, k.Desc)
			delete(ks, "default")
			addPath(s, strings.Split(al.Path, "."), ks)
		}
	}
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["title"] = a.m.AppName + " configuration"
	s["description"] = a.m.AppDesc
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// addPath puts leaf at path in the object schema s, creating objects on the
// way; an existing key is left alone.
func addPath(s map[string]any, path []string, leaf map[string]any) {
	props := s["properties"].(map[string]any)
	if len(path) == 1 {
		if _, ok := props[path[0]]; !ok {
			props[path[0]] = leaf
		}
		return
	}
	next, ok := props[path[0]].(map[string]any)
	if !ok {
		next = map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
		props[path[0]] = next
	}
	if _, isObj := next["properties"]; !isObj {
		return // a key, not a group
	}
	addPath(next, path[1:], leaf)
}

// toAny is an enum's values, []string, as []any.
func toAny(v any) []any {
	if xs, ok := v.([]string); ok {
		out := make([]any, len(xs))
		for i, x := range xs {
			out[i] = x
		}
		return out
	}
	return v.([]any)
}

// comment writes text as comment lines at indent, one per line of text.
func comment(b *strings.Builder, indent, text string) {
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		fmt.Fprintf(b, "%s# %s\n", indent, strings.TrimRight(l, " \t"))
	}
}

func mdCell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}

func code(s string) string {
	if s == "" {
		return ""
	}
	return "`" + strings.ReplaceAll(s, "`", "'") + "`"
}

func typeText(k *KeyModel) string {
	switch {
	case len(k.Enum) > 0:
		texts := make([]string, len(k.Enum))
		for i, e := range k.Enum {
			texts[i] = code(e.Text)
		}
		return "one of " + strings.Join(texts, ", ")
	case k.ir.Kind == ir.KindSecret:
		return "secret"
	case k.ir.Kind == ir.KindSecretFile:
		return "secret file"
	}
	return k.FlagType
}

func defaultText(k *KeyModel) string {
	switch {
	case k.DefaultFrom != "":
		return "from " + code(k.DefaultFrom)
	case k.HasDefault:
		return code(k.DefaultText)
	}
	return ""
}

// configMD is the configuration reference, docs/CONFIG.md. It lists every
// key, hidden ones included.
func (a *artifacts) configMD() []byte {
	m := a.m
	var b strings.Builder
	fmt.Fprintf(&b, "# %s configuration\n\n", m.AppName)
	fmt.Fprintf(&b, "%s. Generated by cfgx from the schema; don't edit.\n\n", strings.TrimSuffix(m.AppDesc, "."))
	b.WriteString("Each setting comes from, highest first: a flag, an environment variable, ")
	if m.ConfigFile != "" {
		fmt.Fprintf(&b, "the config file (`%s.yaml`, `.yml` or `.json`", m.ConfigFile)
		if len(m.SearchPaths) > 0 {
			paths := make([]string, len(m.SearchPaths))
			for i, p := range m.SearchPaths {
				paths[i] = code(p)
			}
			fmt.Fprintf(&b, ", looked for in %s, or named by `--config` or `%s_CONFIG`", strings.Join(paths, ", "), m.EnvPrefix)
		}
		b.WriteString("), ")
	}
	b.WriteString("or its default. An empty value counts as not set.\n")
	for _, g := range m.Groups {
		if len(g.Keys) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n%s.\n\n", code(g.Path), strings.TrimSuffix(g.Desc, "."))
		b.WriteString("| Key | Description | Type | Default | Env | Flag | Used by |\n")
		b.WriteString("|-----|-------------|------|---------|-----|------|---------|\n")
		for _, k := range g.Keys {
			desc := strings.TrimSuffix(k.Desc, ".") + "."
			var notes []string
			if k.Required {
				notes = append(notes, "Required.")
			}
			if k.Deprecated != "" {
				notes = append(notes, "Deprecated: "+strings.TrimSuffix(k.Deprecated, ".")+".")
			}
			for _, al := range k.Aliases {
				notes = append(notes, "Formerly "+code(al.Path)+".")
			}
			if k.Hidden {
				notes = append(notes, "Hidden from `--help`.")
			}
			if k.NoFile {
				notes = append(notes, "Not read from the config file.")
			}
			if k.ir.Example != nil {
				notes = append(notes, "Example: "+code(k.Example)+".")
			}
			if len(notes) > 0 {
				desc += " " + strings.Join(notes, " ")
			}
			env := code(k.Env)
			if k.FileEnv && k.Env != "" {
				env += " or " + code(k.Env+"_FILE")
			}
			flag := ""
			if k.Flag != "" {
				flag = code("--" + k.Flag)
				if k.Short != "" {
					flag = code("-"+k.Short) + ", " + flag
				}
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n", code(k.Path), mdCell(desc), mdCell(typeText(k)),
				mdCell(defaultText(k)), env, flag, mdCell(a.usedBy[k.Index]))
		}
	}
	if len(m.Rules) > 0 || slices.ContainsFunc(m.Keys, func(k *KeyModel) bool { return len(k.RelevantIf) > 0 }) {
		b.WriteString("\n## Rules\n\n")
		for _, r := range m.Rules {
			names := make([]string, len(r.Keys))
			for i, idx := range r.Keys {
				names[i] = code(m.Keys[idx].Path)
			}
			switch r.Kind {
			case "AtMostOne":
				fmt.Fprintf(&b, "- At most one of %s is set.\n", strings.Join(names, ", "))
			case "ExactlyOne":
				fmt.Fprintf(&b, "- Exactly one of %s is set.\n", strings.Join(names, ", "))
			case "AllOrNone":
				fmt.Fprintf(&b, "- Either all of %s are set, or none.\n", strings.Join(names, ", "))
			case "Requires":
				fmt.Fprintf(&b, "- If %s is set, %s must be too.\n", names[0], names[1])
			}
		}
		for _, k := range m.Keys {
			for _, c := range k.RelevantIf {
				fmt.Fprintf(&b, "- %s only matters when %s is set.\n", code(k.Path), code(m.Keys[c].Path))
			}
		}
	}
	var withArgs []*CommandModel
	for _, c := range m.Commands {
		if len(c.Args) > 0 {
			withArgs = append(withArgs, c)
		}
	}
	if len(withArgs) > 0 {
		b.WriteString("\n## Command arguments\n\nThese are flags only, for one command each.\n")
		for _, c := range withArgs {
			fmt.Fprintf(&b, "\n### %s\n\n| Flag | Description | Type | Default |\n|------|-------------|------|---------|\n", c.Name)
			for _, k := range c.Args {
				desc := strings.TrimSuffix(k.Desc, ".") + "."
				if k.Required {
					desc += " Required."
				}
				fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", code("--"+k.Flag), mdCell(desc), mdCell(typeText(k)), mdCell(defaultText(k)))
			}
		}
	}
	return []byte(b.String())
}

// Lock is cfgx.lock.json: the schema's IR without source positions, which
// cfgx diff compares between versions (M6.3).
func Lock(app *ir.App) ([]byte, error) {
	b, err := json.Marshal(app)
	if err != nil {
		return nil, err
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var strip func(v any)
	strip = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			delete(v, "pos")
			delete(v, "storePos")
			delete(v, "problems")
			// A Secret's default is in the schema's source, but the lock
			// file is for reading in PRs: it only says there is one.
			if v["kind"] == string(ir.KindSecret) && v["default"] != nil && v["default"] != "" {
				v["default"] = "******"
			}
			for _, x := range v {
				strip(x)
			}
		case []any:
			for _, x := range v {
				strip(x)
			}
		}
	}
	strip(v)
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
