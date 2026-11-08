# Status log

Newest first. Every working session adds an entry: what landed, the decisions
taken for you, what's next, and what's waiting on you. The terms are in
[ownership.md](ownership.md).

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
- **Next:** M5.3 (rules), M5.4 and M5.5 (to Gemini, Flash or Argon Medium),
  M5.6 (computed methods) and M5.7 (checks); the Argon High second review of
  M3 once its quota resets.
- **Waiting on you:** nothing.

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
