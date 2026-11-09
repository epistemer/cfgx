package gen

import (
	"go/token"
	"regexp"
	"slices"

	"github.com/epistemer/cfgx/internal/ir"
)

var envRE = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// naming applies k's name overrides (M4.9) to km, which has its derived
// names; setGo replaces the key's segment of its Go names. It reports
// whether they were all valid.
func naming(km *KeyModel, k ir.Key, app *ir.App, bad func(ir.Pos, string, ...any), setGo func(string)) bool {
	ok := true
	if k.Flag != "" {
		if !flagRE.MatchString(k.Flag) {
			bad(k.Pos, "flag name %q of %s must start with a-z and be lower-case words of a-z and 0-9 joined by single dashes, without the leading --", k.Flag, km.Path)
			ok = false
		}
		km.Flag = k.Flag
	}
	if k.Env != "" {
		if !envRE.MatchString(k.Env) {
			bad(k.Pos, "env var name %q of %s must be upper-case letters, digits and _, not starting with a digit", k.Env, km.Path)
			ok = false
		}
		km.Env = k.Env
	}
	if k.GoName != "" {
		if !token.IsIdentifier(k.GoName) || !token.IsExported(k.GoName) {
			bad(k.Pos, "GoName %q of %s must be an exported Go identifier, such as Addr", k.GoName, km.Path)
			ok = false
		}
		setGo(k.GoName)
	}
	if k.FileKey != "" && !keyNameRE.MatchString(k.FileKey) {
		bad(k.Pos, "Key %q of %s must be lower-case words of a-z and 0-9 joined by single underscores, with dots between path segments", k.FileKey, km.Path)
		ok = false
	}
	if k.Short != "" {
		r := k.Short[0]
		if len(k.Short) != 1 || !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			bad(k.Pos, "Short %q of %s must be one ASCII letter or digit", k.Short, km.Path)
			ok = false
		}
		km.Short = k.Short
	}
	return ok
}

// afterKind applies what depends on the kind too: Sources, Hidden and
// Example, and checks the names cfgx reserves. It reports whether the key
// is usable.
func afterKind(km *KeyModel, k ir.Key, app *ir.App, prefix string, bad func(ir.Pos, string, ...any)) bool {
	if k.Kind == ir.KindSecret && (k.Flag != "" || k.Short != "") {
		bad(k.Pos, "%s is a Secret, which never has a flag, since a flag's value shows in ps and shell history", km.Path)
		return false
	}
	if k.Sources != nil {
		if !slices.Contains(k.Sources, "flag") {
			km.Flag = ""
		}
		if !slices.Contains(k.Sources, "env") {
			km.Env = ""
		}
		if !slices.Contains(k.Sources, "file") {
			km.NoFile = true
		}
		if app.ConfigFile == "" && km.Flag == "" && km.Env == "" && !km.NoFile {
			bad(k.Pos, "%s can only come from the config file, but the app has none (App.ConfigFile)", km.Path)
		}
		if (k.Flag != "" || k.Short != "") && km.Flag == "" {
			bad(k.Pos, "%s names a flag, but Sources leaves out FromFlag", km.Path)
		}
		if k.Env != "" && km.Env == "" {
			bad(k.Pos, "%s names an env var, but Sources leaves out FromEnv", km.Path)
		}
	}
	if km.Short != "" && km.Flag == "" {
		bad(k.Pos, "%s has a short flag but no flag", km.Path)
	}
	if k.Complete != nil && km.Flag == "" {
		bad(k.Pos, "%s has a completion but no flag", km.Path)
	}
	switch {
	case km.Flag == "help":
		bad(k.Pos, "flag --help of %s collides with the built-in help flag", km.Path)
	case app.ConfigFile != "" && km.Flag == "config":
		bad(k.Pos, "flag --config of %s collides with the config file flag", km.Path)
	}
	if km.Short == "h" {
		bad(k.Pos, "short flag -h of %s collides with the built-in help flag", km.Path)
	}
	if app.ConfigFile != "" && km.Env == prefix+"_CONFIG" {
		bad(k.Pos, "env var %s of %s collides with the env var that names the config file", km.Env, km.Path)
	}
	km.Hidden = k.Hidden
	if k.Example != nil {
		text := *k.Example
		if spec, ok := kinds[k.Kind]; ok && spec.canon != nil {
			t, _, err := spec.canon(text)
			if err != nil {
				bad(k.Pos, "example of %s: %v", km.Path, err)
				return false
			}
			text = t
		} else if k.Kind == ir.KindEnum && !slices.ContainsFunc(km.Enum, func(e EnumModel) bool { return e.Text == text }) {
			bad(k.Pos, "example %q of %s isn't one of its values", text, km.Path)
			return false
		}
		km.Example = text
	}
	return true
}
