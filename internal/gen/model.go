package gen

import (
	"fmt"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/epistemer/cfgx/internal/ir"
)

// Model is the IR plus every derived name, ready to render.
type Model struct {
	Package  string
	Keys     []*KeyModel
	Groups   []*GroupModel
	Commands []*CommandModel
}

// KeyModel is one key with its derived names.
type KeyModel struct {
	Index                    int
	Path, Desc               string
	Field, Getter, Flag, Env string
	GoType, FlagType, Parse  string
	HasDefault               bool
	DefaultText, DefaultLit  string
	Quote, Bool              bool
}

// GroupModel is one group: its interface and the type that implements it.
type GroupModel struct {
	Name, Desc  string
	Iface, Type string
	Keys        []*KeyModel
}

// CommandModel is one command profile.
type CommandModel struct {
	Name, Lower string
	Groups      []*GroupModel
	KeyIdx      []int
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

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// initialisms are written in capitals in Go names (D4).
var initialisms = map[string]string{
	"api": "API", "db": "DB", "dns": "DNS", "http": "HTTP", "https": "HTTPS", "id": "ID",
	"ip": "IP", "json": "JSON", "jwt": "JWT", "mfa": "MFA", "otp": "OTP", "smtp": "SMTP",
	"sql": "SQL", "ssh": "SSH", "tcp": "TCP", "tls": "TLS", "ttl": "TTL", "udp": "UDP",
	"uri": "URI", "url": "URL", "uuid": "UUID", "xml": "XML", "yaml": "YAML",
}

func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '.' || r == '-' })
}

// pascal turns a schema name into a Go name: database_url → DatabaseURL.
func pascal(s string) string {
	var b strings.Builder
	for _, w := range words(s) {
		if up, ok := initialisms[w]; ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	return b.String()
}

// lowerFirstWord lower-cases a Go name's first word: DB → db,
// SMTPHost → smtpHost, Database → database (interface §5.3).
func lowerFirstWord(s string) string {
	r := []rune(s)
	n := 0
	for n < len(r) && unicode.IsUpper(r[n]) {
		n++
	}
	switch {
	case n == 0:
	case n == 1 || n == len(r):
		for i := 0; i < n; i++ {
			r[i] = unicode.ToLower(r[i])
		}
	default: // an initialism followed by a word: keep the word's capital
		for i := 0; i < n-1; i++ {
			r[i] = unicode.ToLower(r[i])
		}
	}
	return string(r)
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
	var probs Problems
	bad := func(pos ir.Pos, format string, args ...any) {
		probs = append(probs, Problem{pos, fmt.Sprintf(format, args...)})
	}
	if app.Name == "" {
		bad(app.Pos, "the app needs a name")
	}
	if strings.TrimSpace(app.Desc) == "" {
		bad(app.Pos, "the app needs a description")
	}
	prefix := envPrefix(app)
	if prefix != "" && !regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`).MatchString(prefix) {
		bad(app.Pos, "env prefix %q must be upper-case letters, digits and underscores", prefix)
	}

	m := &Model{Package: pkg}
	// Every generated identifier, to catch collisions between them.
	idents := map[string]ir.Pos{"values": app.Pos, "keys": app.Pos, "root": app.Pos, "BindRoot": app.Pos}
	claim := func(name string, pos ir.Pos, what string) {
		if prev, ok := idents[name]; ok {
			bad(pos, "%s %s collides with another generated name (from %s)", what, name, prev)
			return
		}
		idents[name] = pos
	}
	flags, envs, fields := map[string]ir.Pos{}, map[string]ir.Pos{}, map[string]ir.Pos{}
	groups := map[string]*GroupModel{}

	for _, g := range app.Groups {
		if !nameRE.MatchString(g.Name) {
			bad(g.Pos, "group name %q must start with a lower-case letter and use only a-z, 0-9 and _", g.Name)
			continue
		}
		if _, dup := groups[g.Name]; dup {
			bad(g.Pos, "group %q is declared twice", g.Name)
			continue
		}
		if strings.TrimSpace(g.Desc) == "" {
			bad(g.Pos, "group %q needs a description", g.Name)
		}
		gm := &GroupModel{Name: g.Name, Desc: g.Desc, Iface: pascal(g.Name)}
		gm.Type = lowerFirstWord(gm.Iface) + "Group"
		claim(gm.Iface, g.Pos, "group interface")
		claim(gm.Type, g.Pos, "group type")
		groups[g.Name] = gm
		m.Groups = append(m.Groups, gm)

		seen := map[string]bool{}
		for _, k := range g.Keys {
			if !nameRE.MatchString(k.Name) {
				bad(k.Pos, "key name %q must start with a lower-case letter and use only a-z, 0-9 and _", k.Name)
				continue
			}
			if seen[k.Name] {
				bad(k.Pos, "key %q is declared twice in group %q", k.Name, g.Name)
				continue
			}
			seen[k.Name] = true
			if strings.TrimSpace(k.Desc) == "" {
				bad(k.Pos, "key %s.%s needs a description", g.Name, k.Name)
			}
			km := &KeyModel{Index: len(m.Keys), Path: g.Name + "." + k.Name, Desc: k.Desc}
			km.Field = pascal(g.Name) + pascal(k.Name)
			km.Getter = km.Field
			km.Flag = strings.NewReplacer(".", "-", "_", "-").Replace(km.Path)
			km.Env = prefix + "_" + strings.ToUpper(strings.ReplaceAll(km.Path, ".", "_"))
			if err := kindInfo(km, k); err != nil {
				bad(k.Pos, "%v", err)
				continue
			}
			for _, c := range []struct {
				seen map[string]ir.Pos
				name string
				what string
			}{{flags, km.Flag, "flag --"}, {envs, km.Env, "env var "}, {fields, km.Field, "Go name "}} {
				if prev, ok := c.seen[c.name]; ok {
					bad(k.Pos, "%s%s of %s collides with another key's (declared at %s)", c.what, c.name, km.Path, prev)
				}
				c.seen[c.name] = k.Pos
			}
			gm.Keys = append(gm.Keys, km)
			m.Keys = append(m.Keys, km)
		}
	}

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
		cm := &CommandModel{Name: c.Name, Lower: lowerFirstWord(c.Name)}
		claim(cm.Name, c.Pos, "command type")
		claim(cm.Name+"Command", c.Pos, "command binder")
		claim(cm.Lower+"Keys", c.Pos, "command key list")
		claim("new"+cm.Name, c.Pos, "command constructor")
		used := map[string]bool{}
		for _, u := range c.Uses {
			gm, ok := groups[u]
			if !ok {
				bad(c.Pos, "command %s uses group %q, which isn't in the dsl.App", c.Name, u)
				continue
			}
			if used[u] {
				bad(c.Pos, "command %s uses group %q twice", c.Name, u)
				continue
			}
			used[u] = true
			cm.Groups = append(cm.Groups, gm)
			for _, k := range gm.Keys {
				cm.KeyIdx = append(cm.KeyIdx, k.Index)
			}
		}
		sort.Ints(cm.KeyIdx)
		m.Commands = append(m.Commands, cm)
	}

	if len(probs) > 0 {
		return nil, probs
	}
	return m, nil
}

func kindInfo(km *KeyModel, k ir.Key) error {
	km.HasDefault = k.Default != nil
	if km.HasDefault {
		km.DefaultText = *k.Default
	}
	switch k.Kind {
	case ir.KindString:
		km.GoType, km.FlagType, km.Parse, km.Quote = "string", "string", "ParseString", true
		km.DefaultLit = strconv.Quote(km.DefaultText)
	case ir.KindInt:
		km.GoType, km.FlagType, km.Parse = "int", "int", "ParseInt"
		if km.HasDefault {
			if _, err := strconv.Atoi(km.DefaultText); err != nil {
				return fmt.Errorf("default %q of %s isn't an int", km.DefaultText, km.Path)
			}
		}
		km.DefaultLit = km.DefaultText
	case ir.KindBool:
		km.GoType, km.FlagType, km.Parse, km.Bool = "bool", "bool", "ParseBool", true
		if km.HasDefault {
			if _, err := strconv.ParseBool(km.DefaultText); err != nil {
				return fmt.Errorf("default %q of %s isn't a bool", km.DefaultText, km.Path)
			}
		}
		km.DefaultLit = km.DefaultText
	default:
		return fmt.Errorf("key %s has kind %q, which this cfgx doesn't support", km.Path, k.Kind)
	}
	return nil
}
