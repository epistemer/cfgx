package engine

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/storage"
)

// Writer is the admin API that cfgx/store builds its Store on.
var _ storage.Admin = (*Writer[struct{}])(nil)

func (w *Writer[V]) adminKey(i int) storage.KeyInfo {
	k := &w.a.Keys[i]
	return storage.KeyInfo{Path: k.Path, Desc: k.Desc, Kind: k.Kind, Mutable: k.Mutable, Default: k.DefaultText,
		Enum: slices.Clone(k.Enum), Schema: k.Schema, Tags: slices.Clone(k.Tags)}
}

// AdminKeys implements storage.Admin.
func (w *Writer[V]) AdminKeys() []storage.KeyInfo {
	out := make([]storage.KeyInfo, len(w.idx))
	for j, i := range w.idx {
		out[j] = w.adminKey(i)
	}
	return out
}

// AdminSettings implements storage.Admin.
func (w *Writer[V]) AdminSettings() []storage.Setting {
	w.mu.Lock()
	res := w.res
	w.mu.Unlock()
	v := res.Values
	out := make([]storage.Setting, len(w.idx))
	for j, i := range w.idx {
		k := &w.a.Keys[i]
		s := storage.Setting{KeyInfo: w.adminKey(i), Source: "unset", Invalid: res.Invalid[i]}
		if k.Mutable && k.Format != nil {
			if text, err := k.Format(v); err == nil {
				s.Value = text
			}
		} else {
			s.Value = plainText(k.Show(v))
		}
		if og := res.Origins[i]; len(og) > 0 {
			top := og[len(og)-1]
			s.Source = sourceText(top)
			if top.Layer == LayerStore {
				for _, o := range og[:len(og)-1] {
					if o.Layer != LayerDefault {
						s.Shadowed = append(s.Shadowed, sourceText(o))
					}
				}
			}
		}
		if r, ok := res.Records[i]; ok {
			s.Record = &r
		}
		out[j] = s
	}
	return out
}

// sourceText names an origin for the admin API: a file position gets
// "file " in front.
func sourceText(o Origin) string {
	if o.Layer == LayerFile {
		return "file " + o.Where
	}
	return o.String()
}

// key finds the command's key at path.
func (w *Writer[V]) key(path string) (int, error) {
	for _, i := range w.idx {
		if w.a.Keys[i].Path == path {
			return i, nil
		}
	}
	return 0, fmt.Errorf("%s isn't a key of this command", path)
}

// WriteText implements storage.Admin. Keys are written in sorted order, so the
// history doesn't depend on map order.
func (w *Writer[V]) WriteText(ctx context.Context, set map[string]string, reset []string, options ...cfgx.WriteOption) error {
	type change struct {
		i     int
		raw   string
		reset bool
	}
	var cs []change
	for _, p := range slices.Sorted(maps.Keys(set)) {
		i, err := w.key(p)
		if err != nil {
			return err
		}
		cs = append(cs, change{i: i, raw: set[p]})
	}
	for _, p := range reset {
		i, err := w.key(p)
		if err != nil {
			return err
		}
		if _, both := set[p]; both {
			return fmt.Errorf("%s is both set and reset", p)
		}
		cs = append(cs, change{i: i, reset: true})
	}
	return w.Write(ctx, func(e *Edits[V]) error {
		for _, c := range cs {
			if c.reset {
				e.Reset(c.i)
			} else {
				e.SetText(c.i, c.raw)
			}
		}
		return nil
	}, options...)
}

// History implements storage.Admin.
func (w *Writer[V]) History(key string, limit int) ([]storage.Record, error) {
	if _, err := w.key(key); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.backend == nil {
		return nil, errClosed
	}
	return w.backend.History(key, limit)
}

// Backend implements storage.Admin.
func (w *Writer[V]) Backend() storage.Backend {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.backend
}

// plainText is a shown value as text: a string as it is, the rest as JSON.
func plainText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return jsonText(v)
}
