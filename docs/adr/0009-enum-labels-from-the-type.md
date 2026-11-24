# ADR 0009: An enum's values take their labels from their type

| | |
|---|---|
| Status | Accepted (amendment A16, locked by epistemer on 2026-11-24) |
| Date | 2026-11-24 |
| Roadmap item | M12.2 |
| Deciders | epistemer chose the direction on 2026-11-24 (question C2 of Confab iteration 1). Claude proposed the test and its records. It changes ADR 0008 for one case |

## Context

Under [ADR 0008](0008-labels-as-data.md) (A14), `App.Labels()` gives every
enum value a label, `settings.<key path>.<value>`, whose source text is the
value. Confab, the first app on all of neozp, declares
`dsl.Enum("default_level", …, models.TalkLevelBeginner, …)` over an sqlc
enum. models already labels that type's values, `enum.talk_level.beginner`
and so on, and its `Label()` returns them (finding F3):

- a translator translates "beginner" twice;
- a settings page that shows cfgx's labels disagrees with every other page
  once one of the two is translated;
- Confab's settings form uses `models.AllTalkLevelValues()`, so cfgx's value
  labels are dead keys.

cfgx doesn't import i18n (ADR 0008), and the producer of a label never
derives another producer's key names (neozp's labels-as-data rules), so
cfgx can't point at models' keys.

## Options

1. **Keep A14:** every value gets a cfgx label. Two translations per value.
2. **Skip value labels when the type labels itself.** The DSL sees `T`
   through reflection, so it can tell at schema time.
3. **An opt-out per key**, such as `.NoValueLabels()`. One more DSL
   construct, and every app with a labelled type must remember it.
4. **Publish the typed values in `store.KeyInfo`** (`[]any`), so a generic
   dashboard could ask each value for its label. More API, and no app needs
   it yet.

## Decision

Option 2. An `Enum` whose `T` has, in its value method set, a method
`Label()` with no parameters and one result gets no value labels; the key's
own label stays. The test is structural, so it holds for models' enums (an
`option.SelectOption`, whose `Label()` returns an `i18n.TKer`) without cfgx
importing models or i18n. A `Label() string` counts too: such a type labels
its own values.

## Consequences

- The DSL records the fact in the IR (`EnumLabeled`), and `gen` leaves those
  values out of `zz_<package>.labels.json`. The IR version goes up.
- `L10n(path)` returns nil for such a value's path. A generic dashboard falls
  back to the value's text; interface §5.5's example says so. Option 4 was
  asked as question C2b and answered no for now; it stays open for when a
  generic dashboard needs the type's labels.
- Translations stored for `settings.<key>.<value>` of such a key become i18n
  orphans until `Prune`. Confab's are dead keys already.
- Every other neozp generator whose enums have `Label()` (models today) is
  covered without a change on its side.
