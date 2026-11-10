package dsl

import (
	"fmt"
	"slices"

	"github.com/epistemer/cfgx/internal/ir"
)

// Source is a place a value can come from, for Sources.
type Source int

// The sources of a value. (dsl.File is the File key constructor, so these
// are FromFlag, FromEnv and FromFile.)
const (
	FromFlag Source = iota + 1
	FromEnv
	FromFile
)

func shortText(r rune) string {
	if r == 0 {
		return ""
	}
	return string(r)
}

// Short gives the key's flag a one-letter form: Short('p') adds -p.
func (k *key[K, T, G]) Short(r rune) K {
	k.short = r
	return k.self
}

// Flag replaces the key's derived flag name, such as --server-port, whole.
func (k *key[K, T, G]) Flag(name string) K {
	k.flag = name
	return k.self
}

// Env replaces the key's derived env var name, such as ZP_SERVER_PORT,
// whole: the app's prefix isn't added.
func (k *key[K, T, G]) Env(name string) K {
	k.env = name
	return k.self
}

// Key replaces the key's own name in the config file, the last segment of
// its path, keeping the name it was declared with for its env var, flag
// and getter.
func (k *key[K, T, G]) Key(name string) K {
	k.fileKey = name
	return k.self
}

// GoName replaces the key's segment of its Go names: in group server,
// GoName("Addr") gives ServerAddr().
func (k *key[K, T, G]) GoName(name string) K {
	k.goName = name
	return k.self
}

// Sources limits where the value may come from: Sources(FromFile, FromEnv)
// gives the key no flag. A key that can't come from the config file and is
// found there is ignored, with a warning.
func (k *key[K, T, G]) Sources(sources ...Source) K {
	k.sources = []string{}
	if len(sources) == 0 {
		k.problems = append(k.problems, "Sources needs at least one of FromFlag, FromEnv and FromFile")
	}
	for _, s := range sources {
		var name string
		switch s {
		case FromFlag:
			name = "flag"
		case FromEnv:
			name = "env"
		case FromFile:
			name = "file"
		default:
			k.problems = append(k.problems, fmt.Sprintf("Sources: unknown source %d", s))
			continue
		}
		if !slices.Contains(k.sources, name) {
			k.sources = append(k.sources, name)
		}
	}
	if k.kind == ir.KindSecret {
		for _, s := range k.sources {
			if s == "flag" {
				k.problems = append(k.problems, "a Secret never has a flag, since a flag's value shows in ps and shell history")
			}
		}
	}
	return k.self
}

// Hidden leaves the key out of --help and example files. It still works,
// and the generated docs list it.
func (k *key[K, T, G]) Hidden() K {
	k.hidden = true
	return k.self
}

// Example is shown in example files instead of the default.
func (k *key[K, T, G]) Example(v T) K {
	s := k.text(v)
	k.example = &s
	return k.self
}

// Mutable lets the key change at runtime: the app writes it with Update or
// Store().Update, the value is kept in the store (App.Store), and it wins
// over flags, env vars, the config file and the default (interface §4).
func (k *key[K, T, G]) Mutable() K {
	k.mutable = true
	return k.self
}

// Epoch sets the key's epoch, starting at 0. Raising it makes stored values
// written under an older epoch go to history unused, for a release that
// changes what the key means.
func (k *key[K, T, G]) Epoch(n int) K {
	if n < 0 {
		k.problems = append(k.problems, fmt.Sprintf("Epoch(%d) must not be negative", n))
	}
	k.epoch = n
	return k.self
}
