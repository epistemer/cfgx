package gen

import (
	"fmt"
	"go/token"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/epistemer/cfgx/internal/ir"
)

// Model is the IR plus every derived name, ready to render.
type Model struct {
	Package     string
	EnvPrefix   string
	ConfigFile  string
	SearchPaths []string
	Formats     []string
	GlobalIdx   []int // indexes of the keys in Global groups
	Keys        []*KeyModel
	Groups      []*GroupModel
	Tags        []*TagModel
	Rules       []*RuleModel
	Commands    []*CommandModel
	Imports     []ImportModel // for the getters' types and computed methods
	// Warnings are schema issues that don't stop generation.
	Warnings Problems

	imp  *importer // names imports; ResolveRefs updates them
	refs []pkgRef  // packages imported for keys, for ResolveRefs
}

// RuleModel is a cross-key rule, by key index.
type RuleModel struct {
	Kind  string // the engine constant: AtMostOne, ...
	Group string
	Keys  []int
}

// TagModel is one tag: an interface over keys from any group.
type TagModel struct {
	Name, Desc string
	Keys       []*KeyModel
}

// AliasModel is one alias for a key across file, env, and flag.
type AliasModel struct {
	Path, Env, Flag string
}

// KeyModel is one key with its derived names.
type KeyModel struct {
	Index                    int
	Path, Desc               string
	Field, Getter, Flag, Env string
	GoType, FlagType, Parse  string
	IsSet                    string // Go expression on v: is the value non-zero?
	HasDefault               bool
	DefaultText, DefaultLit  string
	Quote, Bool              bool
	Required, Global         bool
	FlagOnly                 bool
	FileEnv, File, Resolve   bool  // see engine.Key
	Items, Map               bool  // a list or map key: SetItems, see engine.Key
	MaxFileSize              int64 // see engine.Key
	Tags                     []string
	Aliases                  []AliasModel
	Deprecated               string
	RelevantIf               []int // keys that must be set for this one to matter
	// ParseArgs are arguments the parser takes between raw and the field,
	// each followed by ", ".
	ParseArgs string
	// Enum lists an enum's values; EnumTexts and EnumValues name the
	// generated tables of their texts and Go values.
	Enum                  []EnumModel
	EnumTexts, EnumValues string
	zero                  string // the canonical text of the zero value
	// Checks are Go expressions of type error, run in order on a set
	// value; Patterns are the compiled patterns they use. Validate runs
	// after them.
	Checks      []string
	Patterns    []PatternModel
	Validate    *FuncModel
	DefaultFrom string     // the DefaultFunc's name, for --help
	defaultFunc *FuncModel // the DefaultFunc, if any
	// M4.9: Short is the one-letter flag; Hidden leaves the flag out of
	// --help; NoFile keeps the key out of the config file (Sources);
	// Example is the text example files show.
	Short    string
	Hidden   bool
	NoFile   bool
	Example  string
	declPath string // the path the schema declared, which rules name
	// Complete is set when the schema gave the flag a completion: file
	// extensions, directories, or a function.
	Complete     bool
	CompleteExts []string
	CompleteDirs bool
	CompleteFunc *FuncModel
	Pos          ir.Pos
}

// EnumModel is one value of an enum key.
type EnumModel struct{ Text, Lit string }

// fileEnv is env's _FILE variant if km has one, else "".
func fileEnv(km *KeyModel, env string) string {
	if !km.FileEnv || env == "" {
		return ""
	}
	return env + "_FILE"
}

// nonZeroDefault reports whether the key's default counts as set (A4).
func (k *KeyModel) nonZeroDefault() bool {
	return k.defaultFunc != nil || k.HasDefault && k.DefaultText != k.zero
}

// GroupModel is one group: its interface and the type that implements it.
type GroupModel struct {
	Name, Path, Desc string
	Iface, Type      string
	Keys             []*KeyModel   // the group's own keys
	Children         []*GroupModel // nested groups, embedded in this one
	AllKeys          []*KeyModel   // own keys and every descendant's, in table order
	Parent           *GroupModel
	Computed         []*ComputedModel
	lower            string // Type without "Group": dbGroup → db
	flagPrefix       string
	pos              ir.Pos
}

// ComputedModel is a hand-written method that joins a group's interface.
// Sig is empty until ResolveComputed fills it in from the type-checked
// package; until then the interface leaves the method out.
type ComputedModel struct {
	Name, Sig string
	Pos       ir.Pos
}

// ImportModel is a package the generated file imports for keys' types and
// computed methods' signatures. Pkg is the package's own name.
type ImportModel struct{ Alias, Path, Pkg string }

// Std reports whether the package is in the standard library, whose
// imports go in the first group.
func (i ImportModel) Std() bool {
	first, _, _ := strings.Cut(i.Path, "/")
	return !strings.Contains(first, ".")
}

// Spec is the import line: the path, with the alias unless it is the
// package's own name and the path's last element, as readers expect.
func (i ImportModel) Spec() string {
	if i.Alias == i.Pkg && i.Pkg == path.Base(i.Path) {
		return strconv.Quote(i.Path)
	}
	return i.Alias + " " + strconv.Quote(i.Path)
}

// CommandModel is one command profile.
type CommandModel struct {
	Name    string
	KeysVar string // the generated slice of the command's key indexes
	Groups  []*GroupModel
	// Forward are keys the command reaches only through a tag: it gets
	// forwarding getters for them instead of embedding their group (A5).
	Forward []*KeyModel
	Args    []*KeyModel
	KeyIdx  []int
}

// HasRoot reports whether the command type needs the values at all.
func (c *CommandModel) HasRoot() bool {
	return len(c.Groups) > 0 || len(c.Forward) > 0 || len(c.Args) > 0
}

// Problem is a schema error found at generate time.
type Problem struct {
	Pos ir.Pos
	Msg string
}

// Problems is every schema error, reported together.
type Problems []Problem

func (ps Problems) Error() string {
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = fmt.Sprintf("%s: %s", p.Pos, p.Msg)
	}
	return strings.Join(lines, "\n")
}

// nameRE matches a name segment: lower-case words joined by single underscores.
// reservedMethods are methods generated on command types, now (Args, A12) or
// in later milestones (M7). A getter or computed method with one of these
// names would be silently shadowed, so the generator rejects it.
var reservedMethods = map[string]bool{"Args": true, "Store": true, "Update": true, "Close": true, "OnChange": true, "Snapshot": true}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// initialisms are written in capitals in Go names (D4).
var initialisms = map[string]string{
	"api": "API", "db": "DB", "dns": "DNS", "http": "HTTP", "https": "HTTPS", "id": "ID",
	"ip": "IP", "json": "JSON", "jwt": "JWT", "mfa": "MFA", "otp": "OTP", "smtp": "SMTP",
	"sql": "SQL", "ssh": "SSH", "tcp": "TCP", "tls": "TLS", "ttl": "TTL", "udp": "UDP",
	"uri": "URI", "url": "URL", "uuid": "UUID", "xml": "XML", "yaml": "YAML",
}

var (
	initialismRE = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	flagRE       = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	// keyNameRE is a key name: name segments joined by dots, a path shorthand.
	keyNameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*(\.[a-z][a-z0-9]*(_[a-z0-9]+)*)*$`)
)

func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '.' || r == '-' })
}

// namer turns schema names into Go names, with the built-in initialisms plus
// the App's.
type namer struct{ init map[string]string }

func newNamer() *namer {
	n := &namer{init: map[string]string{}}
	for k, v := range initialisms {
		n.init[k] = v
	}
	return n
}

// pascal turns a schema name into a Go name: database_url → DatabaseURL.
func (n *namer) pascal(s string) string {
	var b strings.Builder
	for _, w := range words(s) {
		if up, ok := n.init[w]; ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	return b.String()
}

// lowerCamel turns a schema name into an unexported Go name: the first word
// lower-cased, the rest as in pascal. smtp_tls → smtpTLS, db_1 → db1
// (interface §5.3).
func (n *namer) lowerCamel(s string) string {
	ws := words(s)
	if len(ws) == 0 {
		return ""
	}
	return ws[0] + n.pascal(strings.Join(ws[1:], "_"))
}

// lowerGo lower-cases the first word of a Go name given by GoName: DB → db,
// HTTPServer → httpServer, Mail → mail.
func lowerGo(s string) string {
	r := []rune(s)
	n := 0 // the leading run of capitals, digits included after the first
	for n < len(r) && (unicode.IsUpper(r[n]) || n > 0 && unicode.IsDigit(r[n])) {
		n++
	}
	if n == len(r) {
		return strings.ToLower(s) // DB, DB1
	}
	if n > 1 && unicode.IsLower(r[n]) {
		n-- // HTTPServer: keep the S of Server
	}
	return strings.ToLower(string(r[:n])) + string(r[n:])
}

func dashed(s string) string { return strings.NewReplacer(".", "-", "_", "-").Replace(s) }

func joinDash(a, b string) string {
	if a == "" {
		return b
	}
	return a + "-" + b
}

func envPrefix(app *ir.App) string {
	if app.EnvPrefix != "" {
		return app.EnvPrefix
	}
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return r
		}
		return '_'
	}, app.Name))
}

// Build checks the schema and derives every name. All problems are reported
// together, each at its schema position.
func Build(app *ir.App, pkg string) (*Model, error) {
	var probs, warnings Problems
	bad := func(pos ir.Pos, format string, args ...any) {
		probs = append(probs, Problem{pos, fmt.Sprintf(format, args...)})
	}
	warn := func(pos ir.Pos, format string, args ...any) {
		warnings = append(warnings, Problem{pos, fmt.Sprintf(format, args...)})
	}
	if app.Name == "" {
		bad(app.Pos, "the app needs a name")
	}
	if strings.TrimSpace(app.Desc) == "" {
		bad(app.Pos, "the app needs a description")
	}
	prefix := envPrefix(app)
	if (app.Name != "" || app.EnvPrefix != "") && !regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`).MatchString(prefix) {
		if app.EnvPrefix == "" {
			bad(app.Pos, "the env prefix derived from the app name %q is %q, which isn't valid; set one with .EnvPrefix, starting with A-Z and using only A-Z, 0-9 and single underscores", app.Name, prefix)
		} else {
			bad(app.Pos, "env prefix %q must start with A-Z and use only A-Z, 0-9 and single underscores", prefix)
		}
	}
	if !token.IsIdentifier(pkg) || token.IsKeyword(pkg) || pkg == "_" {
		bad(app.Pos, "the generated code's package name %q isn't a valid Go package name", pkg)
	}

	m := &Model{Package: pkg, EnvPrefix: prefix, ConfigFile: app.ConfigFile, SearchPaths: app.SearchPaths, Formats: app.Formats}
	checkConfigFile(app, bad)
	// Every generated identifier, to catch collisions between them.
	// Later milestones generate ConfigCommand (M6.4), Test, All and Values
	// (M6.5); they are claimed now so no schema that generates today breaks
	// when they arrive.
	idents := map[string]ir.Pos{"values": app.Pos, "keys": app.Pos, "root": app.Pos, "app": app.Pos, "BindRoot": app.Pos,
		"ConfigCommand": app.Pos, "Test": app.Pos, "All": app.Pos, "Values": app.Pos}
	claim := func(name string, pos ir.Pos, what string) {
		if prev, ok := idents[name]; ok {
			bad(pos, "%s %s collides with another generated name (from %s)", what, name, prev)
			return
		}
		idents[name] = pos
	}
	nm := newNamer()
	imp := newImporter(app.Schema)
	m.imp = imp
	for _, w := range app.Initialisms {
		if !initialismRE.MatchString(w) {
			bad(app.Pos, "initialism %q must be upper-case letters and digits, starting with a letter, like OTP", w)
			continue
		}
		nm.init[strings.ToLower(w)] = w
	}
	flags, envs, fields, paths := map[string]ir.Pos{}, map[string]ir.Pos{}, map[string]ir.Pos{}, map[string]ir.Pos{}
	shorts := map[string]ir.Pos{}
	groups := map[string]*GroupModel{}
	var groupPaths []string

	tagSet := map[string]*TagModel{}
	for _, t := range app.Tags {
		if !token.IsIdentifier(t.Name) || !token.IsExported(t.Name) {
			bad(t.Pos, "tag name %q must be an exported Go identifier, like Mailer; it names the tag's interface", t.Name)
			// Known, so its keys don't report it as undeclared too.
			tagSet[t.Name] = &TagModel{Name: t.Name}
			continue
		}
		if _, dup := tagSet[t.Name]; dup {
			bad(t.Pos, "tag %s is declared twice", t.Name)
			continue
		}
		if strings.TrimSpace(t.Desc) == "" {
			bad(t.Pos, "tag %s needs a description", t.Name)
		}
		tm := &TagModel{Name: t.Name, Desc: t.Desc}
		claim(t.Name, t.Pos, "tag interface")
		tagSet[t.Name] = tm
		m.Tags = append(m.Tags, tm)
	}
	tagPos := map[string]ir.Pos{}
	for _, t := range app.Tags {
		tagPos[t.Name] = t.Pos
	}

	var walk func(g ir.Group, parent *GroupModel, inherited []string) *GroupModel
	walk = func(g ir.Group, parent *GroupModel, inherited []string) *GroupModel {
		for _, t := range g.Tags {
			if _, ok := tagSet[t]; !ok {
				bad(g.Pos, "group %s has tag %s, which isn't declared", g.Name, t)
			}
		}
		inherited = append(append([]string(nil), inherited...), g.Tags...)
		if !nameRE.MatchString(g.Name) {
			bad(g.Pos, "group name %q must start with a lower-case letter and use only a-z, 0-9 and _", g.Name)
			return nil
		}
		path := g.Name
		if parent != nil {
			path = parent.Path + "." + g.Name
		}
		if _, dup := groups[path]; dup {
			bad(g.Pos, "group %q is declared twice", path)
			return nil
		}
		if strings.TrimSpace(g.Desc) == "" {
			bad(g.Pos, "group %q needs a description", path)
		}
		seg := nm.pascal(g.Name)
		if g.GoName != "" {
			if !token.IsIdentifier(g.GoName) || !token.IsExported(g.GoName) {
				bad(g.Pos, "GoName %q of group %q must be an exported Go identifier, like DB", g.GoName, path)
			}
			seg = g.GoName
		}
		gm := &GroupModel{Name: g.Name, Path: path, Desc: g.Desc, Parent: parent, pos: g.Pos}
		seenComputed := map[string]bool{}
		for _, c := range g.Computed {
			switch {
			case !token.IsIdentifier(c) || !token.IsExported(c):
				bad(g.Pos, "computed method %q of group %q must be an exported Go method name, like DSN", c, path)
			case reservedMethods[c]:
				bad(g.Pos, "computed method %s of group %q is reserved for a method on command types", c, path)
			case seenComputed[c]:
				bad(g.Pos, "group %q lists computed method %s twice", path, c)
			default:
				seenComputed[c] = true
				gm.Computed = append(gm.Computed, &ComputedModel{Name: c, Pos: g.Pos})
			}
		}
		switch {
		case parent != nil:
			gm.Iface = parent.Iface + seg
			gm.lower = parent.lower + seg
		case g.GoName != "":
			gm.Iface, gm.lower = seg, lowerGo(seg)
		default:
			gm.Iface, gm.lower = seg, nm.lowerCamel(g.Name)
		}
		gm.Type = gm.lower + "Group"
		switch {
		case g.FlagPrefix != nil:
			gm.flagPrefix = *g.FlagPrefix
			if gm.flagPrefix != "" && !flagRE.MatchString(gm.flagPrefix) {
				bad(g.Pos, "FlagPrefix %q of group %q must be lower-case words joined by single dashes, or \"\"", gm.flagPrefix, path)
			}
		case parent != nil:
			gm.flagPrefix = joinDash(parent.flagPrefix, dashed(g.Name))
		default:
			gm.flagPrefix = dashed(g.Name)
		}
		claim(gm.Iface, g.Pos, "group interface")
		claim(gm.Type, g.Pos, "group type")
		groups[path] = gm
		groupPaths = append(groupPaths, path)
		paths[path] = g.Pos
		m.Groups = append(m.Groups, gm)

		seen := map[string]bool{}
		for _, k := range g.Keys {
			if !keyNameRE.MatchString(k.Name) {
				bad(k.Pos, "key name %q must be lower-case words of a-z and 0-9 joined by single underscores, with dots between path segments", k.Name)
				continue
			}
			if seen[k.Name] {
				bad(k.Pos, "key %q is declared twice in group %q", k.Name, path)
				continue
			}
			seen[k.Name] = true
			km := &KeyModel{Index: len(m.Keys), Path: path + "." + k.Name, Desc: k.Desc, Pos: k.Pos}
			km.declPath = km.Path
			if strings.TrimSpace(k.Desc) == "" {
				bad(k.Pos, "key %s needs a description", km.Path)
			}
			km.Field = gm.Iface + nm.pascal(k.Name)
			km.Getter = km.Field
			km.Flag = joinDash(gm.flagPrefix, dashed(k.Name))
			km.Env = prefix + "_" + strings.ToUpper(strings.ReplaceAll(km.Path, ".", "_"))
			if !naming(km, k, app, bad, func(goName string) { km.Field = gm.Iface + goName; km.Getter = km.Field }) {
				continue
			}
			if k.FileKey != "" {
				km.Path = path + "." + k.FileKey
			}
			if reservedMethods[km.Getter] {
				bad(k.Pos, "getter %s of %s is reserved for a method on command types; rename the key", km.Getter, km.Path)
			}
			if len(k.Problems) > 0 {
				for _, p := range k.Problems {
					bad(k.Pos, "%s: %s", km.Path, p)
				}
				continue
			}
			if err := kindInfo(km, k, imp); err != nil {
				bad(k.Pos, "%v", err)
				continue
			}
			m.keyExtras(km, k, imp, bad)
			km.Required = k.Required
			for _, t := range k.Tags {
				if _, ok := tagSet[t]; !ok {
					bad(k.Pos, "key %s has tag %s, which isn't declared", km.Path, t)
				}
			}
			for _, t := range append(append([]string(nil), inherited...), k.Tags...) {
				if !slices.Contains(km.Tags, t) {
					km.Tags = append(km.Tags, t)
				}
			}
			if !afterKind(km, k, app, prefix, bad) {
				continue
			}
			for _, c := range []struct {
				seen map[string]ir.Pos
				name string
				what string
			}{{flags, km.Flag, "flag --"}, {shorts, km.Short, "short flag -"}, {envs, km.Env, "env var "}, {envs, fileEnv(km, km.Env), "env var "}, {fields, km.Field, "Go name "}, {paths, km.Path, "file key "}} {
				if c.name == "" {
					continue
				}
				if prev, ok := c.seen[c.name]; ok {
					bad(k.Pos, "%s%s of %s collides with another key or group (declared at %s)", c.what, c.name, km.Path, prev)
				}
				c.seen[c.name] = k.Pos
			}
			if k.Deprecated != "" {
				if strings.TrimSpace(k.Deprecated) == "" {
					bad(k.Pos, "deprecated message of %s must not be blank", km.Path)
				}
				km.Deprecated = k.Deprecated
			}
			for _, alias := range k.Aliases {
				segs := strings.Split(alias, ".")
				valid := len(segs) > 1
				if valid {
					for _, s := range segs {
						if !nameRE.MatchString(s) {
							valid = false
							break
						}
					}
				}
				if !valid {
					bad(k.Pos, "alias %q must be a full dotted path whose segments start with a lower-case letter and use only a-z, 0-9 and _", alias)
					continue
				}
				aliasEnv := prefix + "_" + strings.ToUpper(strings.ReplaceAll(alias, ".", "_"))
				aliasFlag := ""
				if km.Flag != "" {
					aliasFlag = dashed(alias)
				}
				for _, c := range []struct {
					seen map[string]ir.Pos
					name string
					what string
				}{{flags, aliasFlag, "flag --"}, {envs, aliasEnv, "env var "}, {envs, fileEnv(km, aliasEnv), "env var "}, {paths, alias, "file key "}} {
					if c.name == "" {
						continue
					}
					if prev, ok := c.seen[c.name]; ok {
						bad(k.Pos, "%s%s of %s collides with another key or group (declared at %s)", c.what, c.name, km.Path, prev)
					}
					c.seen[c.name] = k.Pos
				}
				km.Aliases = append(km.Aliases, AliasModel{
					Path: alias,
					Env:  aliasEnv,
					Flag: aliasFlag,
				})
			}
			gm.Keys = append(gm.Keys, km)
			m.Keys = append(m.Keys, km)
		}
		gm.AllKeys = append(gm.AllKeys, gm.Keys...)
		sibling := map[string]bool{}
		for _, c := range g.Groups {
			if seen[c.Name] || sibling[c.Name] {
				bad(c.Pos, "group %q has two members named %q", path, c.Name)
				continue
			}
			sibling[c.Name] = true
			if cm := walk(c, gm, inherited); cm != nil {
				gm.Children = append(gm.Children, cm)
				gm.AllKeys = append(gm.AllKeys, cm.AllKeys...)
			}
		}
		return gm
	}
	for _, g := range app.Groups {
		walk(g, nil, nil)
	}
	for _, k := range m.Keys {
		for _, t := range k.Tags {
			if tm := tagSet[t]; tm != nil {
				tm.Keys = append(tm.Keys, k)
			}
		}
	}
	for _, tm := range m.Tags {
		if len(tm.Keys) == 0 {
			bad(tagPos[tm.Name], "tag %s isn't on any key; tag keys or groups with .Tags(%s)", tm.Name, tm.Name)
		}
	}
	// A computed method can't share a name with a getter or another computed
	// method in its group's interface, embedded groups included.
	// Two getters with one name (GoName) are reported as a Go name
	// collision, so only repeats beyond the getters count; check every
	// group, since a nested group's computed method can clash with a
	// getter or computed method of an ancestor.
	reported := map[string]bool{}
	for _, gm := range m.Groups {
		names := methodNames(gm)
		getters := map[string]int{}
		for _, k := range gm.AllKeys {
			getters[k.Getter]++
		}
		sorted := make([]string, 0, len(names))
		for name, n := range names {
			if n > 1 && n > getters[name] && !reported[name] {
				sorted = append(sorted, name)
			}
		}
		sort.Strings(sorted)
		for _, name := range sorted {
			reported[name] = true
			bad(gm.pos, "group %q's interface has two methods named %s, at least one computed; rename the computed method", gm.Path, name)
		}
	}
	// A key can't sit where the file needs a map: tls and tls.enabled can't
	// both be keys, and a key can't share its path with a group.
	keyPaths := map[string]bool{}
	for _, k := range m.Keys {
		keyPaths[k.Path] = true
		for _, a := range k.Aliases {
			keyPaths[a.Path] = true
		}
	}
	checkMap := func(path string) {
		segs := strings.Split(path, ".")
		for i := 1; i < len(segs); i++ {
			if p := strings.Join(segs[:i], "."); keyPaths[p] {
				bad(paths[path], "file key %s needs %s to be a map, but %s is a key", path, p, p)
			}
		}
	}
	for _, k := range m.Keys {
		checkMap(k.Path)
		for _, a := range k.Aliases {
			checkMap(a.Path)
		}
	}

	checkRules(app, m, bad)

	var globals []*GroupModel
	isGlobal := map[string]bool{}
	for _, o := range app.GlobalOrphans {
		bad(app.Pos, "Global lists group %q (declared at %s), which isn't passed to dsl.App", o.Name, o.Pos)
	}
	for _, name := range app.Global {
		gm, ok := groups[name]
		if !ok {
			bad(app.Pos, "Global lists group %q, which isn't in the dsl.App", name)
			continue
		}
		if isGlobal[name] {
			bad(app.Pos, "Global lists group %q twice", name)
			continue
		}
		isGlobal[name] = true
		globals = append(globals, gm)
		for _, k := range gm.AllKeys {
			k.Global = true
			m.GlobalIdx = append(m.GlobalIdx, k.Index)
		}
	}
	sort.Ints(m.GlobalIdx)

	cmdNames := map[string]bool{}
	for _, c := range app.Commands {
		if !token.IsIdentifier(c.Name) || !token.IsExported(c.Name) {
			bad(c.Pos, "command name %q must be an exported Go identifier, like Serve", c.Name)
			continue
		}
		if cmdNames[c.Name] {
			bad(c.Pos, "command %s is declared twice", c.Name)
			continue
		}
		cmdNames[c.Name] = true
		cm := &CommandModel{Name: c.Name, KeysVar: "keys" + c.Name}
		claim(cm.Name, c.Pos, "command type")
		claim(cm.Name+"Command", c.Pos, "command binder")
		claim(cm.KeysVar, c.Pos, "command key list")
		claim("new"+cm.Name, c.Pos, "command constructor")
		claim("Load"+cm.Name, c.Pos, "command loader (M6.6)")
		for _, o := range c.Orphans {
			bad(c.Pos, "command %s uses group %q (declared at %s), which isn't passed to dsl.App", c.Name, o.Name, o.Pos)
		}
		// Every command gets the Global groups; using one explicitly as well
		// is harmless. A group whose ancestor is also used is reached through
		// that ancestor, so it isn't embedded a second time.
		var chosen []*GroupModel
		used := map[string]bool{}
		for _, gm := range globals {
			used[gm.Path] = true
			chosen = append(chosen, gm)
		}
		for _, u := range c.Uses {
			gm, ok := groups[u]
			if !ok {
				bad(c.Pos, "command %s uses group %q, which isn't in the dsl.App", c.Name, u)
				continue
			}
			if isGlobal[u] {
				warn(c.Pos, "command %s uses group %q, which is Global: every command gets it already, so drop it from Uses", c.Name, u)
				continue
			}
			if used[u] {
				bad(c.Pos, "command %s uses group %q twice", c.Name, u)
				continue
			}
			used[u] = true
			chosen = append(chosen, gm)
		}
		inKeys := map[int]bool{}
		for _, gm := range chosen {
			if hasUsedAncestor(gm, used) {
				continue
			}
			cm.Groups = append(cm.Groups, gm)
			for _, k := range gm.AllKeys {
				if !inKeys[k.Index] {
					inKeys[k.Index] = true
					cm.KeyIdx = append(cm.KeyIdx, k.Index)
				}
			}
		}
		// Keys reached only through a tag get forwarding getters, in table
		// order, and no computed methods (A5).
		usedTag := map[string]bool{}
		for _, t := range c.Tags {
			tm, ok := tagSet[t]
			if !ok {
				bad(c.Pos, "command %s uses tag %s, which isn't declared", c.Name, t)
				continue
			}
			if usedTag[t] {
				bad(c.Pos, "command %s uses tag %s twice", c.Name, t)
				continue
			}
			usedTag[t] = true
			for _, k := range tm.Keys {
				if !inKeys[k.Index] {
					inKeys[k.Index] = true
					cm.KeyIdx = append(cm.KeyIdx, k.Index)
					cm.Forward = append(cm.Forward, k)
				}
			}
		}
		sort.Slice(cm.Forward, func(i, j int) bool { return cm.Forward[i].Index < cm.Forward[j].Index })
		sort.Ints(cm.KeyIdx)

		cmdFlags := map[string]ir.Pos{}
		cmdGetters := map[string]ir.Pos{}
		cmdShorts := map[string]ir.Pos{}
		for _, idx := range cm.KeyIdx {
			k := m.Keys[idx]
			cmdFlags[k.Flag] = k.Pos
			cmdGetters[k.Getter] = k.Pos
			// The engine also binds the flags of RelevantIf conditions.
			for _, c := range k.RelevantIf {
				cmdFlags[m.Keys[c].Flag] = m.Keys[c].Pos
			}
		}
		for _, k := range c.Args {
			if !keyNameRE.MatchString(k.Name) {
				bad(k.Pos, "key name %q must be lower-case words of a-z and 0-9 joined by single underscores, with dots between path segments", k.Name)
				continue
			}
			km := &KeyModel{
				Index:    len(m.Keys),
				Path:     k.Name,
				Desc:     k.Desc,
				FlagOnly: true,
				Pos:      k.Pos,
			}
			if strings.TrimSpace(k.Desc) == "" {
				bad(k.Pos, "key %s needs a description", km.Path)
			}
			km.declPath = km.Path
			km.Getter = nm.pascal(k.Name)
			km.Field = c.Name + km.Getter
			km.Flag = dashed(k.Name)
			km.Env = ""
			switch {
			case k.Env != "" || k.FileKey != "" || k.Sources != nil:
				bad(k.Pos, "argument %s of %s is flag-only, so Env, Key and Sources don't apply", km.Path, c.Name)
				continue
			case !naming(km, k, app, bad, func(goName string) { km.Getter = goName; km.Field = c.Name + goName }):
				continue
			}
			if km.Short != "" {
				if km.Short == "h" {
					bad(k.Pos, "short flag -h of %s collides with the built-in help flag", km.Path)
				} else if prev, ok := shorts[km.Short]; ok {
					bad(k.Pos, "short flag -%s of %s collides with another key's (declared at %s)", km.Short, km.Path, prev)
				} else if prev, ok := cmdShorts[km.Short]; ok {
					bad(k.Pos, "short flag -%s of %s collides with another flag on %s (declared at %s)", km.Short, km.Path, c.Name, prev)
				}
				cmdShorts[km.Short] = k.Pos
			}
			km.Hidden = k.Hidden

			if reservedMethods[km.Getter] {
				bad(k.Pos, "getter %s of %s is reserved for a method on command types; rename the key", km.Getter, km.Path)
			} else if prev, ok := cmdGetters[km.Getter]; ok {
				bad(k.Pos, "getter %s of %s collides with another getter on %s (declared at %s)", km.Getter, km.Path, c.Name, prev)
			}
			cmdGetters[km.Getter] = k.Pos

			if km.Flag == "help" {
				bad(k.Pos, "flag --help of %s collides with the built-in help flag", km.Path)
			} else if app.ConfigFile != "" && km.Flag == "config" {
				bad(k.Pos, "flag --config of %s collides with the config file flag", km.Path)
			} else if prev, ok := cmdFlags[km.Flag]; ok {
				bad(k.Pos, "flag --%s of %s collides with another flag on %s (declared at %s)", km.Flag, km.Path, c.Name, prev)
			}
			cmdFlags[km.Flag] = k.Pos

			if prev, ok := fields[km.Field]; ok {
				bad(k.Pos, "Go name %s of %s collides with another key or group (declared at %s)", km.Field, km.Path, prev)
			}
			fields[km.Field] = k.Pos

			if len(k.Problems) > 0 {
				for _, p := range k.Problems {
					bad(k.Pos, "%s: %s", km.Path, p)
				}
				continue
			}
			if k.Kind == ir.KindSecret {
				bad(k.Pos, "argument %s of %s is a Secret, which never has a flag, and arguments are flag-only", km.Path, c.Name)
				continue
			}
			if err := kindInfo(km, k, imp); err != nil {
				bad(k.Pos, "%v", err)
				continue
			}
			m.keyExtras(km, k, imp, bad)
			km.Required = k.Required
			if len(k.Aliases) > 0 {
				bad(k.Pos, "aliases aren't supported on arguments")
			}
			if k.Deprecated != "" {
				if strings.TrimSpace(k.Deprecated) == "" {
					bad(k.Pos, "deprecated message of %s must not be blank", km.Path)
				}
				km.Deprecated = k.Deprecated
			}
			if len(k.Tags) > 0 {
				bad(k.Pos, "argument %s of %s has tags, but arguments belong to no interface", km.Path, c.Name)
			}

			cm.Args = append(cm.Args, km)
			cm.KeyIdx = append(cm.KeyIdx, km.Index)
			m.Keys = append(m.Keys, km)
		}
		sort.Ints(cm.KeyIdx)
		// Go promotes a method only if one embedded group has it, so a
		// computed method must be unique on the command type.
		// Getter clashes are reported above; this catches computed ones.
		owner := map[string]string{"Args": "the command"}
		computedBy := map[string]bool{}
		for _, k := range cm.Forward {
			owner[k.Getter] = "the command"
		}
		for _, k := range cm.Args {
			owner[k.Getter] = "the command"
		}
		for _, g := range cm.Groups {
			names := methodNames(g)
			computed := computedNames(g)
			sorted := make([]string, 0, len(names))
			for name := range names {
				sorted = append(sorted, name)
			}
			sort.Strings(sorted)
			for _, name := range sorted {
				if prev, ok := owner[name]; ok && prev != g.Path && (computed[name] || computedBy[name]) {
					bad(c.Pos, "command %s gets method %s from both %s and group %q; Go can't promote it, so rename a computed method", c.Name, name, prev, g.Path)
				}
				owner[name] = g.Path
				computedBy[name] = computedBy[name] || computed[name]
			}
		}
		m.Commands = append(m.Commands, cm)
	}

	// The packages the getters' types need, such as time.
	m.Imports = imp.imports(len(m.Commands) > 0)

	if len(probs) > 0 {
		return nil, probs
	}
	m.Warnings = warnings
	return m, nil
}

// checkRules resolves the rules' keys and checks them: the right number of
// distinct keys, no non-zero defaults on exclusive rules (A4), and no cycles
// of RelevantIf.
func checkRules(app *ir.App, m *Model, bad func(ir.Pos, string, ...any)) {
	byPath := map[string]*KeyModel{}
	for _, k := range m.Keys {
		byPath[k.declPath] = k
	}
	for _, r := range app.Rules {
		var keys []*KeyModel
		seen := map[string]bool{}
		ok := true
		for _, p := range r.Keys {
			k := byPath[p]
			if k == nil {
				bad(r.Pos, "%s refers to key %s, which doesn't exist", r.Kind, p)
				ok = false
				continue
			}
			if seen[p] {
				bad(r.Pos, "%s lists %s twice", r.Kind, p)
				ok = false
				continue
			}
			seen[p] = true
			keys = append(keys, k)
		}
		if !ok {
			continue
		}
		switch r.Kind {
		case ir.RuleAtMostOne, ir.RuleExactlyOne, ir.RuleAllOrNone:
			if len(keys) < 2 {
				bad(r.Pos, "%s needs at least two keys", r.Kind)
				continue
			}
			for _, k := range keys {
				if k.nonZeroDefault() {
					bad(r.Pos, "%s can't include %s, whose default %s counts as set; rules are value-based (A4), so drop the default", r.Kind, k.Path, k.DefaultText)
				}
			}
		case ir.RuleRequires, ir.RuleRelevantIf:
			if len(keys) != 2 {
				bad(r.Pos, "%s takes two keys", r.Kind)
				continue
			}
			if r.Kind == ir.RuleRelevantIf {
				keys[0].RelevantIf = append(keys[0].RelevantIf, keys[1].Index)
				continue // not a rule the engine checks: it decides loading
			}
		default:
			bad(r.Pos, "unknown rule %q", r.Kind)
			continue
		}
		rm := &RuleModel{Kind: string(r.Kind), Group: r.Group}
		for _, k := range keys {
			rm.Keys = append(rm.Keys, k.Index)
		}
		m.Rules = append(m.Rules, rm)
	}
	// RelevantIf must not loop: a matters if b is set, b if a is set.
	state := map[int]int{} // 1 visiting, 2 done
	onCycle := -1          // a key on the cycle found
	var visit func(i int) bool
	visit = func(i int) bool {
		switch state[i] {
		case 1:
			onCycle = i
			return false
		case 2:
			return true
		}
		state[i] = 1
		for _, c := range m.Keys[i].RelevantIf {
			if !visit(c) {
				return false
			}
		}
		state[i] = 2
		return true
	}
	for _, k := range m.Keys {
		if !visit(k.Index) {
			p := m.Keys[onCycle].Path
			for _, r := range app.Rules {
				if r.Kind == ir.RuleRelevantIf && len(r.Keys) == 2 && r.Keys[0] == p {
					bad(r.Pos, "RelevantIf rules on %s form a cycle", p)
					break
				}
			}
			return
		}
	}
}

// methodNames counts the methods of a group's interface by name: getters and
// computed methods, its descendants' included.
func methodNames(g *GroupModel) map[string]int {
	n := map[string]int{}
	var add func(g *GroupModel)
	add = func(g *GroupModel) {
		for _, k := range g.Keys {
			n[k.Getter]++
		}
		for _, c := range g.Computed {
			n[c.Name]++
		}
		for _, ch := range g.Children {
			add(ch)
		}
	}
	add(g)
	return n
}

// computedNames is the set of computed methods in a group's interface,
// nested groups included.
func computedNames(g *GroupModel) map[string]bool {
	n := map[string]bool{}
	var add func(g *GroupModel)
	add = func(g *GroupModel) {
		for _, c := range g.Computed {
			n[c.Name] = true
		}
		for _, ch := range g.Children {
			add(ch)
		}
	}
	add(g)
	return n
}

func hasUsedAncestor(g *GroupModel, used map[string]bool) bool {
	for p := g.Parent; p != nil; p = p.Parent {
		if used[p.Path] {
			return true
		}
	}
	return false
}

// checkConfigFile checks App.ConfigFile and App.Formats.
func checkConfigFile(app *ir.App, bad func(ir.Pos, string, ...any)) {
	base := app.ConfigFile
	if base == "" {
		if len(app.SearchPaths) > 0 {
			bad(app.Pos, "ConfigFile needs a basename, such as \"config\"")
		}
		if len(app.Formats) > 0 {
			bad(app.Pos, "Formats has no effect without ConfigFile")
		}
		return
	}
	switch {
	case base == "." || base == ".." || strings.ContainsAny(base, `/\`):
		bad(app.Pos, "ConfigFile basename %q must be a file name without a directory; put directories in the search paths", base)
	case strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".json"):
		bad(app.Pos, "ConfigFile basename %q must not have an extension: cfgx tries .yaml, .yml and .json", base)
	}
	for _, p := range app.SearchPaths {
		if strings.TrimSpace(p) == "" {
			bad(app.Pos, "ConfigFile has an empty search path; use \".\" for the working directory")
		}
	}
	seen := map[string]bool{}
	for _, f := range app.Formats {
		if f != "yaml" && f != "json" {
			bad(app.Pos, "unknown config file format %q; use dsl.YAML or dsl.JSON", f)
		}
		if seen[f] {
			bad(app.Pos, "Formats lists %s twice", f)
		}
		seen[f] = true
	}
}
