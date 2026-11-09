package lockdiff

import (
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/ir"
)

func str(s string) *string { return &s }

func app(keys ...ir.Key) *ir.App {
	return &ir.App{Name: "zp", EnvPrefix: "ZP", Groups: []ir.Group{{Name: "db", Keys: keys}}}
}

func enum(texts ...string) []ir.EnumValue {
	out := make([]ir.EnumValue, len(texts))
	for i, t := range texts {
		out[i] = ir.EnumValue{Text: t}
	}
	return out
}

// One case per kind of change, with its class.
func TestDiff(t *testing.T) {
	port := ir.Key{Name: "port", Desc: "Port", Kind: ir.KindInt, Default: str("5432")}
	mode := ir.Key{Name: "mode", Desc: "Mode", Kind: ir.KindEnum, Type: &ir.Type{Name: "string"}, Enum: enum("a", "b")}
	with := func(k ir.Key, f func(*ir.Key)) ir.Key { f(&k); return k }
	for _, c := range []struct {
		name     string
		old, new *ir.App
		class    Class
		key, msg string
	}{
		{"key removed", app(port, mode), app(port), Breaking, "db.mode", "removed"},
		{"type changed", app(port), app(with(port, func(k *ir.Key) { k.Kind = ir.KindString })), Breaking, "db.port", "type changed from int to string"},
		{"enum value removed", app(mode), app(with(mode, func(k *ir.Key) { k.Enum = enum("a") })), Breaking, "db.mode", `value "b" removed`},
		{"newly required", app(port), app(with(port, func(k *ir.Key) { k.Required = true })), Breaking, "db.port", "now required"},
		{"required key added", app(port), app(port, with(mode, func(k *ir.Key) { k.Required = true })), Breaking, "db.mode", "added, and required"},
		{"env override changed", app(port), app(with(port, func(k *ir.Key) { k.Env = "PGPORT" })), Breaking, "db.port", `env var override changed from "" to "PGPORT"`},
		{"source dropped", app(port), app(with(port, func(k *ir.Key) { k.Sources = []string{"env", "file"} })), Breaking, "db.port", "can no longer come from the command line"},
		{"env prefix", app(port), &ir.App{Name: "zp", EnvPrefix: "ZAYN", Groups: app(port).Groups}, Breaking, "app", "env prefix changed from ZP to ZAYN"},
		{"default changed", app(port), app(with(port, func(k *ir.Key) { k.Default = str("6432") })), Behaviour, "db.port", "default changed from 5432 to 6432"},
		{"default removed", app(port), app(with(port, func(k *ir.Key) { k.Default = nil })), Behaviour, "db.port", "default changed from 5432 to none"},
		{"constraint", app(port), app(with(port, func(k *ir.Key) { k.Checks = []ir.Check{{Kind: ir.CheckMin, Args: []string{"1"}}} })), Behaviour, "db.port", "constraints changed from none to min(1)"},
		{"renamed with alias", app(port), app(with(port, func(k *ir.Key) { k.Name = "server_port"; k.Aliases = []string{"db.port"} })), Behaviour, "db.port", "renamed to db.server_port"},
		{"deprecated", app(port), app(with(port, func(k *ir.Key) { k.Deprecated = "use url" })), Behaviour, "db.port", "deprecated: use url"},
		{"optional key added", app(port), app(port, mode), Safe, "db.mode", "added"},
		{"enum value added", app(mode), app(with(mode, func(k *ir.Key) { k.Enum = enum("a", "b", "c") })), Safe, "db.mode", `value "c" added`},
		{"description changed", app(port), app(with(port, func(k *ir.Key) { k.Desc = "Server port" })), Safe, "db.port", "description changed"},
		{"no longer required", app(with(port, func(k *ir.Key) { k.Required = true })), app(port), Safe, "db.port", "no longer required"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Diff(c.old, c.new)
			for _, ch := range got {
				if ch.Class == c.class && ch.Key == c.key && strings.HasPrefix(ch.Msg, c.msg) {
					return
				}
			}
			t.Errorf("want %s %s: %s; got %v", c.class, c.key, c.msg, got)
		})
	}
	if got := Diff(app(port, mode), app(port, mode)); len(got) != 0 {
		t.Errorf("no change: got %v", got)
	}
}

func TestReport(t *testing.T) {
	got := Report([]Change{{Breaking, "a.b", "removed"}, {Safe, "a.c", "added"}})
	want := "Breaking:\n  a.b: removed\n\nSafe:\n  a.c: added\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
