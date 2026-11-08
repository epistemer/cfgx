# Status log

Newest first. Every working session adds an entry: what landed, the decisions
taken for you, what's next, and what's waiting on you. The terms are in
[ownership.md](ownership.md).

## 2026-11-09: M5.7 generate-time checks

- **Landed:**
  - Every check the generator makes has a bad schema: 63 of them, run
    through the real DSL, loader and generator. Each one asserts the message
    and the schema line it points at.
  - The generator can now warn without failing. The first warning: a
    command that lists a `Global` group in `Uses`.
  - Writing the cases found three problems that were reported twice. All
    three are fixed.
- **Reviewed:** Gemini 3.8 Flash (Medium) on the diff, about 50k tokens
  ([review 0008](reviews/0008-gemini-m5.7-review.md)). Two harness
  improvements came out of it; two findings were rejected.
- **Cost:** this was the cheapest review so far, from a Medium tier and a
  small diff. Writing the 63 cases myself with a script was cheaper than
  briefing Gemini.
- **Next:** M5 is done except M5.8, which needs M4. M4 is the key vocabulary:
  durations, enums, file kinds (secret files, paths, URLs), lists, maps,
  constraints, and naming modifiers.
- **Waiting on you:** nothing.

## 2026-11-09: M5.6 computed methods

- **Landed:** `Group.Computed("DSN")` adds a hand-written method to the
  group's interface (A8):
  - `cfgx gen` type-checks the settings package with an in-memory stub and
    renders each method's exact signature, such as `RetryPolicy()
    (time.Duration, error)`, with the imports it needs (`/v2` paths and
    renamed imports included).
  - A command that reaches the group only through a tag doesn't get the
    method. An end-to-end test checks this with a compile failure.
  - `examples/mini` now declares cfgx as a `tool`, which the roadmap always
    said it did. Without it, `go mod tidy` drops the generator's new
    dependency, `golang.org/x/tools`.
- **Reviewed:** Gemini 3.8 Flash (High) on the diff (about 210k tokens,
  [review 0007](reviews/0007-gemini-m5.6-review.md)). Two fixes came out of
  it: a nested group's computed method could clash with a parent's getter, and
  the overlay path is now always absolute. The other 4 findings were rejected.
  The "read-only" review left a compiled binary in the repo; Claude deleted it,
  and reviews now run from a folder holding only the diff.
- **Decided for you:** the details in implementation.md §2.3:
  - Promoted methods are rejected; declare the method on the group itself.
  - Imports get a number added when their name is taken.
  - Unrelated type errors don't block generation.
- **Next:** M5.7 (the full list of generate-time checks); then M4, the key
  vocabulary. M5.8 needs M4.
- **Waiting on you:** nothing.

## 2026-11-09: M5.4 and M5.5, by Gemini

- **Landed:** through a PR:
  - **M5.5, command `Args`:** flag-only arguments with getters on the command
    type, such as `m.Down()` and `m.Steps()`.
  - **M5.4, `Alias` and `Deprecated`:** an old file key, env var or flag keeps
    working and warns; a deprecated key warns when it is set.
- **Reviewed:** Gemini 3.8 Flash (High) wrote both items, and Claude reviewed
  them. Claude added two checks to M5.5: an argument's flag can't clash with a
  `RelevantIf` condition's flag, and arguments can't have tags. Since Claude
  reviewed Gemini's work, no second-model review was needed.
- **Cost:** about 570k tokens for M5.5 and 510k for M5.4, mostly Gemini
  reading the code and re-running tests. That's high for small items, even on
  Flash. Next time, for items this size, a brief that hands over the exact
  code skeleton and needs less exploration should cost much less. Doing them
  myself is an option too.
- **Decided for you:** the details in implementation.md §2.3c. Among them:
  - Within a layer, the new name wins over an alias.
  - An alias that is used always warns.
  - Argument getters carry no group prefix.
- **Next:** M5.6 (computed methods, a type-checking pass) and M5.7 (the full
  check list), both Claude's; then M4.
- **Waiting on you:** nothing.

## 2026-11-08: M5.3 rules

- **Landed:**
  - `AtMostOne`, `ExactlyOne`, `AllOrNone` and `Requires` are checked after
    merging. They are value-based (A4): a key counts as set when it's non-zero,
    and a Bool only when it's true.
  - `RelevantIf` decides what loads. A key whose condition isn't set keeps its
    default, isn't parsed or checked, and warns if it was set to a value.
  - Rule problems follow interface §8:
    `auth: token and keys are mutually exclusive (token: env ZP_AUTH_TOKEN,
    keys: app.yaml:2:9)`.
  - The PR merges itself once CI passes.
- **Reviewed:** Gemini 3.8 Flash (High) on the non-test diff (about 90k tokens;
  [review 0006](reviews/0006-gemini-m5.3-review.md)). Three findings fixed: a
  command was held to `Required` on a condition it doesn't own; a `RelevantIf`
  cycle was blamed on the wrong key; and an empty value on an irrelevant key
  warned. One finding is deferred to M4.1, and two were false positives.
- **Decided for you:** the rule details in implementation.md §2.3b:
  - A command checks a rule only when it loads every key in it.
  - The conditions of a command's `RelevantIf` keys are loaded for that
    purpose only.
  - Under A4, `Required` together with a non-zero default is useful, so it
    stays allowed.
  - The "no key file on disk" check moves to M4.4, which adds file kinds.
- **Next:** M5.4 (`Alias`, `Deprecated`) and M5.5 (command `Args`), both
  delegated to Gemini, then M5.6 and M5.7.
- **Waiting on you:** nothing.

## 2026-11-08: M4.3, M5.1 and M5.2

- **Landed:** through a PR:
  - `cfgx.Secret` is in package `cfgx` (ADR 0006), and its leak suite now runs
    in CI.
  - Nested groups produce embedded interfaces (`Scheduler` embeds
    `SchedulerRetry`).
  - Dotted key names nest the file key without adding an interface.
  - `Group.GoName`, `Group.FlagPrefix` and `App.Initialisms` work.
  - Tags work: `dsl.Tag`, `.Tags` on keys and groups, and `Uses(tag)`. A
    command that uses only `Mailer` gets exactly the tagged getters (A5).
- **Reviewed:** Gemini 3.8 Flash (High) reviewed the diff for about 150k
  tokens, a third of M3's review. 3 of its 5 findings were real and are fixed
  ([review 0005](reviews/0005-gemini-m4.3-m5.2-review.md)): a nil key
  panicked instead of being reported, `GoName("DB1")` gave `dB1Group`, and an
  undeclared tag was ignored. The other 2 were false positives, already
  covered by tests.
- **Decided for you:** the structure rules in implementation.md §2.3a. Among
  them: a group has one place in the tree; nested groups inherit their
  parent's flag prefix; a tag on no key is an error. Gemini did no
  implementation this round, because M4.3 was a move and M5.1 and M5.2 touch
  the same files; briefing it would have cost more than doing the work.
- **Noticed:** interface §3.1's key `emailfrom` derives `AuthEmailfrom()`,
  while §5.1 shows `AuthEmailFrom()`. M5.8's example needs `email_from` or a
  key `GoName` (M4.9); noted in the roadmap.
- **Changed with your approval:** the repo now allows auto-merge and deletes
  merged branches, and `main` requires the six CI jobs (admins exempt, for
  docs-only commits). From now on each PR gets auto-merge, with merge commits,
  once the cross-model review is done, so a green PR lands without waiting
  for Claude to be prompted.
- **Next:** M5.3 (rules), M5.4 and M5.5 (to Gemini, Flash or Argon Medium),
  M5.6 (computed methods) and M5.7 (checks); the Argon High second review of
  M3 once its quota resets.
- **Waiting on you:** nothing. You deleted the old merged branches; GitHub
  deletes branches after each merge from now on.

## 2026-11-08: M3 config files, errors and provenance

- **Landed:** M3 through a PR. Config files are the lowest explicit layer
  (default < file < env < flag), found as A11 describes, in YAML or JSON
  through one parser. Every problem is reported at once with where it came
  from, in interface §8's format. Unknown keys in the file and unknown `ZP_*`
  env vars are warnings with "did you mean". Warnings go through `slog`, and
  `cfgx.WithLogger` takes them. `Required` is checked only for the running
  command's keys, and `Global` groups become persistent flags.
  `examples/mini` now reads `mini.yaml`.
- **Reviewed:** Claude reviewed Gemini's parser (M3.1) and suggestion package
  (M3.3) and took both as written. Gemini reviewed Claude's code and found 8
  problems; 5 led to fixes ([review 0004](reviews/0004-gemini-m3-review.md)).
  The biggest: an empty `--config` let `ZP_CONFIG` win over a flag, and the
  "required" message could have quoted a secret.
- **Decided for you:** the details in implementation.md §3.4, "[decided, M3]".
  Among them: an app without `ConfigFile` has no `--config` flag; a null value
  in the file counts as absent; `BindRoot` must be called when the app has a
  config file or global groups. Models: Argon High built the parser and Flash
  High the suggestions. Argon (High and Medium) then ran out of quota for about
  six days, so Flash High ran this review instead.
- **Next:** an Argon High second review of M3 once its quota resets; meanwhile
  M4.3 (the `Secret` type) and M5 in lane B, M4.4–M4.6 in lane A.
- **Waiting on you:** nothing.

## 2026-11-08: amendment A12, positional arguments

- **Landed:** you chose the `Args()` method. Every generated command type now
  has `Args() []string`, returning a copy of the arguments cobra parsed; rules
  such as `cobra.ExactArgs(1)` work as before. Recorded as A12 in interface §12,
  with an end-to-end script.
- **Reviewed:** Gemini found five problems in the change, all fixed. The one
  that mattered: a method on a command type would silently shadow a group getter
  with the same name, so `Args` and the method names M7 will add are now
  reserved, checked at generate time.
- **Next:** M3, config files, errors and provenance.
- **Waiting on you:** nothing.

## 2026-11-08: M2 walking skeleton done

- **Landed:** M2 through a PR. The first real code: IR, DSL, engine, generator,
  loader and `cfgx gen`, plus `examples/mini` and end-to-end scripts. A schema
  becomes generated code and a running binary, with default < env < flag.
- **Reviewed:** Claude reviewed Gemini's M2.6 work and fixed the harness's
  hard-coded dependency versions. Gemini found 13 problems in Claude's code, all
  fixed ([review 0003](reviews/0003-gemini-m2-review.md)). The biggest: a bad
  env var failed startup even when a flag overrode it, and a schema without
  commands generated code that didn't compile.
- **Decided for you:** the temporary `main` now lives in the working directory
  and is built, then run (ADR 0002, revision). A warm `cfgx gen` takes about
  0.9 s. Gemini models: Argon High for this milestone's delegation and reviews.
- **Next:** M3, config files, errors and provenance; in parallel, M4.3 (the
  `Secret` type) and M5.
- **Waiting on you:** one API decision: how a command's run function gets its
  positional arguments. A recommendation is in the conversation.

## 2026-11-08: full ownership

- **Changed:** you gave Claude full ownership. Only API design decisions and
  things Claude lacks permission for come back to you; releases, the `cfgx init`
  skeleton and how Gemini is used are now Claude's calls. ownership.md and the
  roadmap say so.

## 2026-11-08: M1 spikes done

- **Landed:** ADRs 0002–0006 on branch `m1-spikes`, merged through a PR. Gemini
  ran S4 and S5; Claude ran S1–S3, reviewed Gemini's work, and re-checked the
  claims the decisions rest on.
- **Reviewed:** Gemini reviewed Claude's ADRs and found 12 problems, all fixed
  before merge ([review 0002](reviews/0002-gemini-m1-review.md)). The biggest:
  pflag could reject a slice flag before cfgx saw it, and the planned JSON
  surrogate rewrite would have corrupted escaped backslashes.
- **Decided for you:**
  - The generator pipeline stays. Its temporary `main` runs through
    `go run -overlay`, so `cfgx gen` never writes to your tree (ADR 0002).
  - Flags use raw custom values, and cfgx won't raise pflag to v1.1.0, which
    garbles cobra's help flag (ADR 0003).
  - Validators are referenced through `FuncForPC`, with strict rules (ADR 0004).
  - YAML and JSON both go through `go.yaml.in/yaml/v4`. Anchors, aliases and
    merge keys are rejected in v0.1.0 (now a backlog item), and JSON surrogate
    pairs are rewritten so valid JSON never fails (ADR 0005).
  - `Secret` uses a closure: one mask for set and unset, no unmarshalers, and
    `Equal` on SHA-256 digests (ADR 0006).
- **Next:** M2, the walking skeleton.
- **Waiting on you:** how often work should happen (unchanged); optionally,
  deleting merged branches.

## 2026-11-07: ownership starts, M0 merged

- **Landed:** M0 is merged into `main` (fast-forward to `237b78c`). CI passed on
  all six jobs, and the e2e script ran on every OS.
- **Decided for you:** the terms in ownership.md. From M1 on, code lands
  through PRs that Claude merges once CI is green and the cross-model review is
  handled.
- **Next:** M1, the five spikes. agy takes S4 (YAML library) and S5 (secret
  leaks); Claude takes S1 (generator pipeline), S2 (flags) and S3 (function
  references).
- **Waiting on you:**
  - How often work should happen: when you ask, or on a schedule.
  - Optional: delete the merged `m0-foundation` branch. The safety classifier
    blocked Claude from deleting it.
