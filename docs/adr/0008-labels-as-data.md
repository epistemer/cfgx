# ADR 0008: Labels as data: `App.Labels` replaces `App.I18n`

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-13 |
| Roadmap item | M11 |
| Deciders | epistemer chose the direction: one generation pipeline over shared metadata (neozp ADR 0001). Claude worked out cfgx's part. This changes the API (amendment A14), so it's epistemer's to overturn |

## Context

Amendment A13 (2026-11-12) made `cfgx gen` write `zz_<package>_i18n.go` when
the schema uses `App.I18n()`: Go code that calls
[epistemer/i18n](https://github.com/epistemer/i18n), rendered by `RenderI18n`
from a template in `internal/gen`. It followed a convention for all of neozp:
every package that generates code generates its locales too.

Looking at cfgx and i18n side by side a day later showed what that convention
costs:

- cfgx's generator holds a template for another package's API. cfgx's module
  doesn't require i18n, so nothing in cfgx pins `i18n.Key` or `i18n.TKer`.
  Only the modules under `examples/` compile the result.
- Every later neozp generator (models, enums, routes) would carry the same
  template. In zp, four generators did.
- cfgx has to manage a file it may not be writing: dropping `App.I18n()`
  leaves a stub, because `go generate` fails when a file it listed disappears.

**[neozp ADR 0001](https://github.com/epistemer/genx/blob/main/docs/adr/0001-generation-pipeline.md)** (2026-11-13)
replaces the convention for the whole framework. In short:

- A generator publishes data files in named, versioned shapes, called
  *domains*. It doesn't write code for another package.
- A package that wants code from that data ships a *trivial generator*: a
  declaration of which domains it reads, queries over them, and templates. It
  contains no code.
- A *driver* runs generators in order, checks for stale output and removes
  leftovers: [genx](https://github.com/epistemer/genx).

That record and the [driver's roadmap](https://github.com/epistemer/genx/blob/main/docs/roadmap.md) live in genx's repository.
Where this ADR repeats part of the record, the record is the reference.

This ADR settles what cfgx publishes.

What cfgx computes for labels is already data. `gen.Build` fills
`Model.L10n` with one `L10nModel{Var, Path, Key, Text}` per group, key and
enum value, in group order, and claims every variable name against the rest
of the generated package.

## Options

1. **Keep A13.** No work now; the costs above stay and repeat in each new
   generator.
2. **Let `cfgx gen` take extra templates over its model.** cfgx would stop
   naming i18n, but it would grow a template host and a public model, and
   every other generator would need the same.
3. **Call `cfgx.lock.json` cfgx's data and let consumers derive labels from
   it.** Nothing changes in cfgx. But the lock is the IR without positions:
   its `version` is the IR version, already 18, and it holds what the schema
   declares, not the getter and interface names the `L10n` variables are built
   from. A consumer would have to repeat cfgx's naming rules (initialisms,
   `GoName`), and nobody could check its names against the generated package.
4. **Publish the label records themselves, in the shared `labels/v1` shape.**
   A small change, since `Model.L10n` already is that list.

## Decision

Option 4.

- `App.Labels()` replaces `App.I18n()`. It stays opt-in, because it reserves
  identifiers in the generated package and writes a file.
- With it, `cfgx gen` writes `zz_<package>.labels.json` next to
  `zz_<package>.go`: a `labels/v1` file with the records A13 computes,
  unchanged and in the same order. `cfgx gen -check` covers it. `-artifacts`
  doesn't move it: it belongs to the package, not to the module root.
- cfgx keeps claiming `L10n`, `l10n` and every `L10n<Name>` in the generated
  package, as it does now, so a clash is still reported at generate time with
  both positions.
- cfgx writes no Go that imports i18n. `RenderI18n`, its template and the stub
  for a dropped `App.I18n()` go away.
- `zz_<package>_i18n.go` is then written by i18n's `labels` generator
  ([i18n ADR 0003](https://github.com/epistemer/i18n/blob/main/docs/adr/0003-labels-generator.md)),
  run by the driver. It keeps its name and its declarations.
- `cfgx.lock.json` stays what it is: cfgx's own snapshot for `cfgx diff`. It
  isn't a domain, and other packages shouldn't read it.
- cfgx publishes no wider "model" domain yet. One can be added when something
  needs more than labels; the admin UI in the roadmap's parking lot is the
  likely first.

### The file

```json
{
  "domain": "labels/v1",
  "producer": "cfgx",
  "package": { "name": "settings", "dir": "internal/settings" },
  "labels": [
    { "key": "settings.server", "text": "HTTP server",
      "path": "server", "var": "L10nServer" },
    { "key": "settings.server.port", "text": "Listen port",
      "path": "server.port", "var": "L10nServerPort" },
    { "key": "settings.log.level.debug", "text": "debug",
      "path": "log.level.debug" }
  ]
}
```

| Field | What cfgx writes |
|-------|------------------|
| `domain` | `labels/v1` |
| `producer` | `cfgx` |
| `package.name` | The generated package's name |
| `package.dir` | Its directory, with forward slashes, relative to the module root (`.` at the root) |
| `labels[].key` | `settings.<path>`; for an enum value, `settings.<key path>.<value>` |
| `labels[].text` | The group's or key's description; for an enum value, the value |
| `labels[].path` | The group's or key's dotted path, as `Store().Keys()` reports it; for an enum value, the key's path, a dot and the value |
| `labels[].var` | `L10n` plus the group's interface name or the key's getter name. Enum values have none |

The order is the one A13 uses: each group, then its keys, each key followed
by its enum values. Command arguments get no labels, as before.

In `labels/v1`, `path` and `var` are optional, the order of `labels` is
kept by whoever renders them, and a renderer that writes Go into the package
may declare each `var`, plus `L10n` and `l10n`, and nothing else at package
level. The shape only gains optional fields within `v1`.

### Evidence

On 2026-11-13 the 62 labels of `examples/zp` (48 with a variable) were
copied by a script from the generated file into a `labels/v1` file, and
rendered with one jq query and one Go template, the prototype of i18n's
generator. The result has the same 117 lines of declarations as the
`zz_settings_i18n.go` that `App.I18n()` writes at commit `5a1759c`; only
comments differ.

### Getting there

`App.I18n()` has never been in a tagged release: `v0.1.0` predates A13. So
there is no deprecation period, only an order that keeps `examples/zp`
building:

1. **M11.1** adds `App.Labels()` and the labels file. `App.I18n()` keeps
   working until the next step.
2. **M11.2** moves `examples/zp` to the driver and i18n's generator, checks
   that the declarations didn't change, and then removes `App.I18n()`.
3. No release is tagged between the two, so no tag ever carries `App.I18n()`.

## Consequences

- cfgx no longer depends on i18n's API in any form. A change there touches
  i18n's own template.
- An app that wants `zz_<package>_i18n.go` needs the driver and i18n's
  generator. cfgx on its own writes the labels file and stops there.
- `cfgx gen` no longer has to leave a stub behind: the driver removes output
  that nothing produces any more.
- The labels file is an interface to other packages. Its shape can't change
  the way `cfgx.lock.json` does.
- The IR's `i18n` flag becomes `labels`, so the IR version becomes 19 and
  every `cfgx.lock.json` changes with it.
- Interface §3.7, §5.5 and §9, implementation §2.5, the README and the
  CHANGELOG are rewritten in M11.2, with the behaviour. Until then they
  describe A13, and amendment A14 in interface §12 records the decision.
- The roadmap gains M11. Its two code items wait on the driver: M11.1 on the
  frozen `labels/v1` shape, M11.2 on the driver's release candidate and on
  i18n's generator.
- The global Claude Code note that every neozp generator emits its locales is
  out of date. Its replacement: a generator writes a `labels/v1` file.
