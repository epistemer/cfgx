# neozp ADR 0001: One generation pipeline over shared metadata

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-13 |
| Scope | neozp as a whole: cfgx, i18n, the generation driver, and every later package that generates code |
| Deciders | epistemer proposed the shared-metadata convention and chose the direction; Claude reviewed it and wrote the details, each of which epistemer can overturn |
| Replaces | The convention recorded with cfgx amendment A13: "every neozp package that generates code generates its locales too" |
| Follow-ups | cfgx [ADR 0008](../adr/0008-labels-as-data.md) and amendment A14; i18n [ADR 0003](https://github.com/epistemer/i18n/blob/main/docs/adr/0003-labels-generator.md); the [driver's roadmap](genx-roadmap.md) |

This record and the driver's roadmap are parked in cfgx's repository, under
`docs/neozp/`, because the driver has no repository yet. They move there when
it has one (driver roadmap M0.4), and the driver's interface document then
becomes the reference for the file formats sketched here.

## Context

zp generates code with the programs under `gen/`, run in a hand-written order
from `generate.go`. Four of them (enum, meta, error, routes) declare i18n keys,
each from its own template.

neozp rebuilds zp's framework package by package. The first two packages, cfgx
(configuration) and i18n (translations), already depend on each other in both
directions:

- **i18n → cfgx.** `i18nschema.Group()` declares i18n's one setting as a cfgx
  schema fragment. It's a separate module, so the i18n core doesn't require
  cfgx.
- **cfgx → i18n.** `App.I18n()` (cfgx amendment A13) makes `cfgx gen` write
  `zz_<package>_i18n.go`, Go code that calls `i18n.Key`, from a template inside
  cfgx.

The first direction is a pattern every later package can copy. The second has
three costs:

1. cfgx's generator names another package's API in a template. cfgx's module
   doesn't require i18n, so nothing pins the API that template targets. Only
   cfgx's example modules would notice a break.
2. A13's convention makes every later generator carry its own copy of the
   labels template, as zp's generators did.
3. No generator sees all the keys, so nothing checks them across packages
   before the app starts.

A framework with several generators also needs what zp's `gen` gave it: one
command that regenerates everything in the right order, and one check for CI.

## Options

| # | Option | For | Against |
|---|--------|-----|---------|
| 1 | Keep A13's convention: every generator writes its own i18n keys | No new tool | The three costs above, once per generator |
| 2 | One `gen` package that knows every component, as in zp | One pipeline | Every new package edits it; cfgx and i18n stop working on their own |
| 3 | Each generator accepts extra templates over its own model | No driver | Every generator grows a template host; a consumer needs one template per producer |
| 4 | Consumers ship generator code that a driver compiles in, the way cfgx loads a schema | Full Go for the consumer | Runs dependency code at generate time; a compile on every run; every package that exports a generator imports the driver |
| 5 | Shared metadata: producers publish data files, consumers ship declarative generators, a driver runs them | Neither side names the other in code; a consumer needs no code and no dependency; one pipeline | A new tool to build; a query language and a template language that must stay small |

## Decision

Option 5, as epistemer proposed it, with the changes the review made (listed
further down).

### 1. Vocabulary

| Term | Meaning |
|------|---------|
| Domain | A named, versioned shape of data, such as `labels/v1` |
| Domain file | A JSON file holding one domain's data from one producer |
| Producer | Any program that writes domain files: `cfgx gen`, sqlc, a later routes generator |
| Trivial generator | A generator that is only data: which domains it reads, queries over them, templates, and where the output goes. It contains no code |
| Metafile | The file in a module that declares that module's trivial generators. Without a driver it does nothing |
| Manifest | The app's file that lists its producers and the trivial generators it has switched on |
| Driver | The program that runs producers and trivial generators in order, checks for stale output, and removes output nobody produces any more. Working name `genx`; the name and the repository are epistemer's to choose |

### 2. Who knows what

- A **producer** knows its own model and the domains it publishes. It never
  names a consumer.
- A **consumer** knows the domains it reads and its own API. It never names a
  producer.
- A **foreign tool**, such as sqlc, knows neither. Whoever wants its data
  writes a trivial generator that turns the tool's output into a domain.
- The **app's manifest** is the only place that names both sides.

### 3. Domains

- A domain is `name/vN`. Within a major version a domain only gains optional
  fields. Removing a field or changing its meaning is a new major version.
- A domain file written for neozp says which domain it is, in a top-level
  `domain` field. A foreign file gets its domain in the manifest.
- Domain files hold **resolved** facts: the Go names, the package, the paths,
  after every naming rule and collision check the producer has. A consumer
  never re-derives a name.
- Domain files are committed, next to the code they describe
  (`zz_<name>.<domain>.json`). They are what `check` compares, and a reviewer
  can read them.
- **Neutral domains for shared concerns.** When several producers feed one
  consumer, they all write the same domain. Labels are the first: every
  producer writes `labels/v1`, and i18n reads only that.
- A producer's private files are not domains. `cfgx.lock.json` is cfgx's IR
  for `cfgx diff`: its format version is already 18, and it holds only what
  the schema declares, not the names cfgx derives.

### 4. `labels/v1`

A label is a piece of text the app shows for something the code names: a
setting, an enum value, a table column.

```json
{
  "domain": "labels/v1",
  "producer": "cfgx",
  "package": { "name": "settings", "dir": "internal/settings" },
  "labels": [
    { "key": "settings.server.port", "text": "Listen port",
      "path": "server.port", "var": "L10nServerPort" },
    { "key": "settings.log.level.debug", "text": "debug",
      "path": "log.level.debug" }
  ]
}
```

| Field | Required | Meaning |
|-------|----------|---------|
| `domain` | yes | `labels/v1` |
| `producer` | yes | Who wrote the file; used in messages |
| `package.name` | yes | The Go package the labels belong to |
| `package.dir` | yes | That package's directory, with forward slashes, relative to the module root |
| `labels[].key` | yes | The text's name across the whole app. One key has one text |
| `labels[].text` | yes | The text in the app's source language |
| `labels[].path` | no | A name for finding the label at run time inside its package. Unique in the package |
| `labels[].var` | no | A Go identifier for the label's handle in the package. Unique in the package |

- The order of `labels` matters: a renderer keeps it.
- **Reserved identifiers.** A renderer that writes Go into `package` may
  declare each label's `var`, plus `L10n` and `l10n`, and nothing else at
  package level. A producer keeps those names free in that package.
- Planned, not in the first cut: an optional `meta` field (base64) for the
  bytes `i18n.Key` accepts, which zp's route labels use.

### 5. Trivial generators

The proposal was a 5-tuple. This is how each part ended up:

| Proposed | Decided | Why |
|----------|---------|-----|
| Selector: `jq` or `yq` | One query language, jq, run by the driver in-process. The file's extension says whether it's JSON or YAML | The choice was about the input's format, not about the query |
| Domain: a list of file paths | `needs`: a list of domains | The metafile ships in one module; the files are in the app, at paths only the app knows |
| Vars: name → query | Kept. A query may return any JSON value, and one more query, `units`, can split the input into several renders | Separate lists of names and descriptions lose which goes with which; several producers can feed one package |
| Templates: Go template files | Kept: `text/template` with no custom functions to start with; `.go` output goes through gofmt | Queries compute, templates print |
| Target: a package | `out`: a query that gives the output path inside the app's module | Only the domain knows where the app keeps the package |

A trivial generator may also write a domain file. That is how a foreign file
becomes a domain.

The field names (`needs`, `units`, `vars`, `out`) are working names. The driver
freezes them after its spikes.

### 6. The driver

- **Switching on is explicit.** The driver can list the metafiles in the
  app's module graph, but it runs only what the manifest names. Adding a
  dependency never adds generated files.
- **Order comes from the domains.** A step that needs a domain runs after the
  steps that write it. zp's hand-written order list goes away.
- **`check` writes nothing.** It fails if any output is stale or left over.
- **The driver deletes what it no longer produces**, if the file still looks
  generated.
- **Files are the protocol.** A consumer that needs real code is an ordinary
  program in the manifest that reads the same domain files.
- The driver itself runs no code from dependencies. Queries can't read the
  environment, the clock or other files, so the same inputs give the same
  output.

### 7. What a trivial generator may not do

- **Parse source, or check one producer's model.** That is a producer's job:
  it writes a program and publishes a domain.
- **Derive Go names from a neozp producer's data.** The domain already holds
  them (§3).
- It **may** assert things no single producer can see, by failing its query
  with a message: one key with two texts, two producers claiming one path.
- It **may** derive names from a foreign tool's data, with functions defined
  in the query. The names can drift from the tool's own rules, so a test pins
  them.

### 8. What changes in the two existing packages

- **cfgx** ([ADR 0008](../adr/0008-labels-as-data.md), A14): `App.Labels()`
  replaces `App.I18n()`. cfgx writes `zz_<package>.labels.json` and no longer
  writes Go that imports i18n.
- **i18n** (ADR 0003, linked above): ships one trivial generator, `labels`,
  that turns every `labels/v1` file into `zz_<package>_i18n.go`.
  `i18nschema` stays as it is.

## What the review changed

The design passed. A second look at both repositories changed eight things,
some of them reversing what Claude had suggested earlier in the same
conversation.

| # | Before | After | Why |
|---|--------|-------|-----|
| 1 | cfgx publishes its model; an adapter turns it into labels | cfgx writes `labels/v1` itself | cfgx already builds exactly these records (`L10nModel`: variable, path, key, text) and checks their names against the rest of the generated package. An adapter would have to re-derive the names and couldn't check them |
| 2 | `cfgx.lock.json` could serve as cfgx's domain | It isn't a domain | Its `version` is the IR version, 18 so far, and it has no resolved names |
| 3 | One render per domain file | One render per unit, and a unit can merge several files | zp's enum and meta generators both write into `models`. Two label files for one package must become one Go file |
| 4 | Nothing said about the names a consumer's template declares | The domain reserves them (§4) | Otherwise i18n's template could collide with a producer's code, and only the compiler would say so |
| 5 | Validation always means a real generator | Checks across files may live in a query | Nobody but the consumer sees every producer's keys |
| 6 | Extract a generator kit from `cfgx/internal` first | Don't | The driver runs no Go from schemas, so it doesn't need cfgx's loader. A kit for writing producers can wait for the second producer written in Go |
| 7 | Build the driver with the third generator (models and enums) | Build it now, with cfgx's labels as the first user | That case already exists with a known-good output, so "same declarations as before" is a test. The sqlc generators come second and prove foreign files |
| 8 | Metafiles could live anywhere in a module | Where they live is a spike | `go mod vendor` copies package directories only. A folder of YAML and templates would not be vendored |

One correction to an earlier claim: the zp generators that declare i18n keys
are enum, meta, error and routes. svg writes only static labels.

## Evidence

Two things were tried on real data on 2026-11-13, with jq 1.7 and Go's
`text/template` and `go/format`, and no custom functions in either:

- **cfgx's labels.** The 62 labels of cfgx's `examples/zp` (48 with a
  variable), copied by a script from cfgx's generated file into a `labels/v1`
  file, were rendered by one jq query and one template. The result has the
  same 117 lines of declarations as the `zz_settings_i18n.go` that
  `App.I18n()` writes; only comments differ. The same query merged two files
  for one package, wrote a second package's file, and stopped with a message
  on a key that had two texts. The query and the template are in i18n's
  ADR 0003.
- **sqlc's enum names.** A one-line `pascal` function written in jq gave the
  same Go constant name as sqlc for all 112 values of zp's 28 enum types.

Not yet tried: the same queries on gojq, the library the driver would embed
(the test used the C implementation); finding metafiles under `go.work` and
vendoring; sqlc's JSON output on the version zp pins. The driver's spikes cover
all three.

## Consequences

- cfgx stops knowing i18n. A change to i18n's API touches one template, which
  ships in i18n at the version the app requires.
- A package that wants generated labels writes a `labels/v1` file and nothing
  else. It needs no template and no i18n import in its generator.
- Label keys are checked across the whole app when code is generated, before
  i18n's `Load` would refuse them at startup.
- An app that wants labels needs the driver. cfgx and i18n still work without
  it; they just don't produce the labels file's Go code on their own.
- There is a new tool to build and keep small. The risk is that queries and
  templates grow into a second programming language. §7 is the guard, and the
  roadmap keeps "no custom functions" as a default that needs a reason to
  break.
- Domain files become interfaces between packages. A producer can't change
  one freely; that is why they're versioned and why private files such as the
  lock are kept out.
- The note in epistemer's global `CLAUDE.md` that every neozp generator emits
  its locales is out of date. Its replacement: a generator writes a
  `labels/v1` file.

## Not settled here

The driver's roadmap settles these, each with a spike or an item:

- The driver's name and repository.
- The file names and field names of the manifest and the metafile.
- Where a metafile lives in a module, and what happens under vendoring.
- How `check` treats a producer that has no check mode of its own.
