# Status log

Newest first. Every working session adds an entry: what landed, the decisions
taken for you, what's next, and what's waiting on you. The terms are in
[ownership.md](ownership.md).

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
