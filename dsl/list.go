package dsl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/epistemer/cfgx/internal/ir"
)

// csvJoin writes items as one CSV record, the text the IR records for a
// list default. An item is quoted when it is empty or holds a comma, a
// quote, a line break or leading space, so it reads back the same.
func csvJoin(items []string) string {
	out := make([]string, len(items))
	for i, it := range items {
		if it == "" || strings.ContainsAny(it, ",\"\r\n") || strings.TrimLeft(it, " \t") != it {
			it = `"` + strings.ReplaceAll(it, `"`, `""`) + `"`
		}
		out[i] = it
	}
	return strings.Join(out, ",")
}

func textAll[T any](text func(T) string) func([]T) string {
	return func(vs []T) string {
		items := make([]string, len(vs))
		for i, v := range vs {
			items[i] = text(v)
		}
		return csvJoin(items)
	}
}

// StringsKey is a list of strings. Create it with Strings.
type StringsKey struct {
	key[*StringsKey, []string, []string]
}

// Strings declares a []string key. A flag may be repeated, and each
// occurrence, like an env var, is a comma-separated list: --tag a --tag b,c
// gives [a b c]. An item holding a comma is quoted: "a,b". A list in the
// config file gives its items as they are. The layer that sets the key
// replaces the whole list; an empty flag or env var clears it.
func Strings(name, desc string) *StringsKey {
	k := &StringsKey{}
	k.init(k, name, desc, ir.KindStrings, csvJoin, caller(1))
	return k
}

// IntsKey is a list of ints. Create it with Ints.
type IntsKey struct{ key[*IntsKey, []int, []int] }

// Ints declares a []int key, set like Strings.
func Ints(name, desc string) *IntsKey {
	k := &IntsKey{}
	k.init(k, name, desc, ir.KindInts, textAll(strconv.Itoa), caller(1))
	return k
}

// DurationsKey is a list of durations. Create it with Durations.
type DurationsKey struct {
	key[*DurationsKey, []time.Duration, []time.Duration]
}

// Durations declares a []time.Duration key, set like Strings; each item
// needs a unit, as for Duration.
func Durations(name, desc string) *DurationsKey {
	k := &DurationsKey{}
	k.init(k, name, desc, ir.KindDurations, textAll(time.Duration.String), caller(1))
	return k
}

// StringMapKey is a map of strings. Create it with StringMap.
type StringMapKey struct {
	key[*StringMapKey, map[string]string, map[string]string]
}

// StringMap declares a map[string]string key. Flags and env vars take
// key=value items, comma-separated, and a flag may be repeated: --label
// a=1,b=2 --label c=3. The config file takes a map. The layer that sets the
// key replaces the whole map; a later item with the same key wins.
func StringMap(name, desc string) *StringMapKey {
	k := &StringMapKey{}
	k.init(k, name, desc, ir.KindStringMap, mapText, caller(1))
	return k
}

// Default sets the key's default. Keys can't be empty or hold an =.
func (k *StringMapKey) Default(m map[string]string) *StringMapKey {
	for key := range m {
		if strings.TrimSpace(key) == "" || strings.Contains(key, "=") || key != strings.TrimSpace(key) {
			k.problems = append(k.problems, fmt.Sprintf("default key %q must be non-empty, without = or outer spaces", key))
		}
	}
	return k.key.Default(m)
}

func mapText(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]string, len(keys))
	for i, key := range keys {
		items[i] = key + "=" + m[key]
	}
	return csvJoin(items)
}
