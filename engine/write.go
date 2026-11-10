package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/spf13/pflag"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/opts"
	"github.com/epistemer/cfgx/internal/storage"
)

// Writer changes the mutable keys of a command that has the store. It is
// the write pipeline that a generated Update and the store's admin API
// share (implementation §3.8):
//
//  1. Build the candidate: the keys load again from the sources they
//     loaded from at startup (the bootstrap), with the stored records as
//     they will be after the change. A reset removes a key's record, so
//     its bootstrap value comes back.
//  2. Validate the whole candidate: every key's checks, Required and the
//     rules, across static and mutable keys.
//  3. Write every change, with its history entry, in one transaction.
//  4. Swap the snapshot.
//
// One mutex is held from step 1 to step 4, so concurrent writers never lose
// each other's changes. An error before step 3 commits leaves the store and
// the snapshot as they were.
type Writer[V any] struct {
	mu   sync.Mutex
	a    *App[V]
	snap Snapshot[V]
	now  func() time.Time

	// The bootstrap: what the keys loaded from at startup. Env lookups
	// and file reads are replayed as they went then.
	idx    []int
	loaded map[int]bool
	file   *configFile
	fs     *pflag.FlagSet
	seen   map[string]envSeen
	reads  map[string]string

	// Guarded by mu: the store, its records as this process last wrote or
	// read them, the ones that failed to decode, and the latest load.
	backend storage.Backend
	records map[string]storage.Record
	bad     map[string]error
	res     *Result[V]

	// OnChange callbacks, the changes queued for them, and whether a
	// writer is running them; also guarded by mu.
	subs     []func(paths []string)
	queue    [][]string
	draining bool
}

// errClosed is what writes to a closed store return.
var errClosed = errors.New("cfgx: the store is closed")

// newWriter keeps what l loaded from, for writes to replay.
func newWriter[V any](l *loader[V], idx []int) *Writer[V] {
	w := &Writer[V]{a: l.a, now: time.Now, idx: idx, loaded: l.loaded, file: l.file, fs: l.fs,
		seen: l.seen, reads: l.reads, backend: l.res.Store, records: l.records, bad: l.badRecords, res: l.res}
	if w.records == nil {
		w.records = map[string]storage.Record{}
	}
	w.snap.Store(l.res.Values)
	return w
}

// Snapshot is the current values, which every getter reads and each
// successful write replaces.
func (w *Writer[V]) Snapshot() *Snapshot[V] { return &w.snap }

// Edits collects the changes of one write, each by key index. A later
// change to a key replaces an earlier one.
type Edits[V any] struct {
	a    *App[V]
	list []edit
	at   map[int]int // key index → its place in list
	err  error
}

type edit struct {
	i     int
	raw   string
	reset bool
}

func (e *Edits[V]) add(ed edit) {
	if ed.i < 0 || ed.i >= len(e.a.Keys) {
		e.fail(fmt.Errorf("cfgx: no key %d", ed.i))
		return
	}
	if j, ok := e.at[ed.i]; ok {
		e.list[j] = ed
		return
	}
	e.at[ed.i] = len(e.list)
	e.list = append(e.list, ed)
}

// Set sets key i to the value assign stores in a scratch V, which is
// written as the key's canonical text (Key.Format) and parsed back like
// any other source.
func (e *Edits[V]) Set(i int, assign func(*V)) {
	if i < 0 || i >= len(e.a.Keys) {
		e.fail(fmt.Errorf("cfgx: no key %d", i))
		return
	}
	k := &e.a.Keys[i]
	if k.Format == nil {
		e.fail(fmt.Errorf("%s isn't mutable", k.Path))
		return
	}
	v := new(V)
	assign(v)
	raw, err := k.Format(v)
	if err != nil {
		e.fail(fmt.Errorf("%s: %v", k.Path, err))
		return
	}
	e.add(edit{i: i, raw: raw})
}

// SetText sets key i to raw, its value as text.
func (e *Edits[V]) SetText(i int, raw string) { e.add(edit{i: i, raw: raw}) }

// Reset removes key i's stored value, so the value it loads from the
// flags, env vars, config file or default applies again.
func (e *Edits[V]) Reset(i int) { e.add(edit{i: i, reset: true}) }

func (e *Edits[V]) fail(err error) {
	if e.err == nil {
		e.err = err
	}
}

// Write runs fill, which collects changes, and applies them together, or
// none of them. A problem with the new values is a cfgx.Errors, as at
// startup. Options say who made the change and why, for the history.
func (w *Writer[V]) Write(ctx context.Context, fill func(*Edits[V]) error, options ...cfgx.WriteOption) error {
	var o opts.Write
	for _, opt := range options {
		if opt != nil {
			opt(&o)
		}
	}
	// fill runs before the lock: it may read getters, or take its time.
	e := &Edits[V]{a: w.a, at: map[int]int{}}
	if err := fill(e); err != nil {
		return err
	}
	if e.err != nil {
		return e.err
	}
	for _, ed := range e.list {
		k := &w.a.Keys[ed.i]
		switch {
		case !k.Mutable:
			return fmt.Errorf("%s isn't mutable", k.Path)
		case !w.loaded[ed.i]:
			return fmt.Errorf("%s isn't a key of this command", k.Path)
		case len(ed.raw) > storage.MaxValue:
			return fmt.Errorf("%s: the value is larger than %s, the most the store keeps", k.Path, FormatByteSize(storage.MaxValue))
		}
	}
	if len(e.list) == 0 {
		return nil
	}

	err := w.commit(ctx, e, o)
	w.drain()
	return err
}

// commit is the locked part of a write: build, validate, store, swap. A
// write that changes values queues their keys for the OnChange callbacks.
func (w *Writer[V]) commit(ctx context.Context, e *Edits[V], o opts.Write) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.backend == nil {
		return errClosed
	}
	recs, bad := maps.Clone(w.records), maps.Clone(w.bad)
	now := w.now().UTC()
	strict := map[int]bool{}
	var changes []storage.Change
	for _, ed := range e.list {
		k := &w.a.Keys[ed.i]
		cur, had := recs[k.Path]
		_, wasBad := bad[k.Path]
		delete(bad, k.Path)
		h := storage.Record{Raw: ed.raw, Epoch: k.Epoch, UpdatedAt: now, UpdatedBy: o.By, Reason: o.Reason}
		if ed.reset {
			if !had && !wasBad {
				continue // nothing stored: nothing changes
			}
			delete(recs, k.Path)
			h.Raw, h.Reset = "", true
			changes = append(changes, storage.Change{Key: k.Path, History: h})
			continue
		}
		// The new value must pass, even if it is the stored one already.
		strict[ed.i] = true
		if had && !wasBad && cur.Raw == ed.raw && cur.Epoch == k.Epoch {
			continue
		}
		recs[k.Path] = h
		r := h
		changes = append(changes, storage.Change{Key: k.Path, Record: &r, History: h})
	}
	reads := maps.Clone(w.reads)
	res, err := w.replay(recs, bad, strict, reads)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.backend.Apply(changes); err != nil {
		return err
	}
	old := w.snap.Load()
	w.records, w.bad, w.res, w.reads = recs, bad, res, reads
	w.snap.Store(res.Values)
	var changed []string
	for _, i := range w.idx {
		k := &w.a.Keys[i]
		if k.Mutable && valueText(k, old) != valueText(k, res.Values) {
			changed = append(changed, k.Path)
		}
	}
	if len(changed) > 0 && len(w.subs) > 0 {
		w.queue = append(w.queue, changed)
	}
	return nil
}

// valueText is k's value in v as text, to compare two values.
func valueText[V any](k *Key[V], v *V) string {
	if k.Format != nil {
		if s, err := k.Format(v); err == nil {
			return s
		}
	}
	return plainText(k.Show(v))
}

// OnChange adds fn to the functions called after a write changes values,
// with the paths of the keys whose values changed, in table order. They
// run one at a time, in the order the writes committed, after the new
// values are in place and the write's lock is released, so fn may write
// too. They run on the goroutine of a writer, not always the one whose
// write changed the values: a write can return before its callbacks run.
func (w *Writer[V]) OnChange(fn func(paths []string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.subs = append(w.subs, fn)
}

// drain runs the queued callbacks, unless another writer is running them
// already, in which case it runs these too.
func (w *Writer[V]) drain() {
	w.mu.Lock()
	if w.draining {
		w.mu.Unlock()
		return
	}
	w.draining = true
	done := false
	defer func() {
		if !done { // a callback panicked: let the next write carry on
			w.mu.Lock()
			w.draining = false
			w.mu.Unlock()
		}
	}()
	for len(w.queue) > 0 {
		paths := w.queue[0]
		w.queue = w.queue[1:]
		subs := w.subs[:len(w.subs):len(w.subs)]
		w.mu.Unlock()
		for _, fn := range subs {
			fn(paths)
		}
		w.mu.Lock()
	}
	w.draining = false
	done = true
	w.mu.Unlock()
}

// replay loads the command's keys again from the bootstrap sources, with
// recs as the stored records. Keys in strict must take their stored value.
// Files read for the first time go into reads, which the write keeps only
// if it commits.
func (w *Writer[V]) replay(recs map[string]storage.Record, bad map[string]error, strict map[int]bool, reads map[string]string) (*Result[V], error) {
	res := &Result[V]{Origins: make([][]Origin, len(w.a.Keys)), File: w.res.File, Store: w.backend, Writer: w}
	l := &loader[V]{a: w.a, res: res, file: w.file, fs: w.fs, reads: reads,
		records: recs, badRecords: bad, strict: strict,
		lookup: func(name string) (string, bool) { s := w.seen[name]; return s.value, s.ok }}
	v := new(V)
	l.loadAll(v, l.start(w.idx))
	if err := l.finish(v); err != nil {
		return nil, err
	}
	return res, nil
}

// Close closes the store; later writes fail. Closing twice is harmless.
func (w *Writer[V]) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.backend == nil {
		return nil
	}
	err := w.backend.Close()
	w.backend = nil
	return err
}
