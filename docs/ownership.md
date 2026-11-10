# Ownership until v1.0.0

| | |
|---|---|
| Owner | Claude, acting for epistemer (delegated on 2026-11-07; full ownership from 2026-11-08) |
| Until | `v1.0.0` is tagged |
| Coding assistant | agy → Gemini 4 Argon (High) |
| Source of truth | [interface.md](interface.md) (the contract), [implementation.md](implementation.md) (how), [roadmap.md](roadmap.md) (order and done-when) |
| Record | [status.md](status.md), plus a PR for every code change |

## What Claude does without asking

- Plans and sequences the roadmap, and splits items between Claude and agy as
  in roadmap.md, "Working with agents".
- Writes code, tests and docs. Delegates leaf items to agy, each in its own git
  worktree outside the repository.
- Has every change reviewed by the other model family: Gemini reviews Claude's
  work, and Claude reviews Gemini's.
- Opens PRs, and merges them into `main` once CI is green and the review
  findings are handled. Docs-only bookkeeping (status log, roadmap ticks) may go
  straight to `main`.
- Decides implementation questions and spike outcomes, each recorded as an ADR.
- Makes amendments that clarify the interface without changing its intent, like
  A4–A11. Each is recorded in interface §12 as decided by Claude and listed in
  the status log, so you can overturn it.
- Adds dependencies an ADR justifies.
- Tags releases, from `v0.1.0` to `v1.0.0`, once a release's done-when is met.
- Manages agy: which tasks Gemini takes, which model and reasoning effort each
  task gets, and when to review or redo its work (see roadmap.md, "Working with
  agents").
- Decides product details that aren't API, such as the `cfgx init` skeleton
  (M8.1).

## What comes back to you

Since 2026-11-08, only two things:

1. **API design decisions.** Anything that changes what a developer writes or
   gets back, beyond a clarification of the locked interface: a new or changed
   DSL construct, generated API or behaviour an application can see, and any
   change to decisions D1–D11. Claude brings a recommendation.
2. **Anything Claude lacks permission for.** Work outside this repository (such as zp
   and its staging and production), your accounts, GitHub settings, tokens and paid services,
   speaking for you to other people, and operations the safety checks block,
   such as deleting branches or rewriting history.

## How you follow along

- [status.md](status.md) gets a dated entry for every working session: what
  landed, decisions taken for you, what's next, and what's waiting on you.
- Every code change lands as a PR on GitHub, with its review notes and CI.
- Roadmap boxes are ticked as items meet their done-when.

## How work continues

Claude doesn't run in the background. Work happens while a session runs: when
you say "continue", or on a schedule if you set one up. Each session starts from
the roadmap, the status log and the memory notes, so any session can pick up
where the last one stopped.

## Limits

- **v1.0.0 is your call.** Tagging it promises the API won't break, so Claude
  doesn't tag it on its own.
- **Quotas.** agy uses your Gemini quota, and Claude's sessions have context
  limits. Large fan-outs check `agy_status` first.
- **Safety checks still apply.** The auto-mode safety classifier blocks some git
  operations, such as deleting remote branches. Claude leaves those to you
  rather than working around them.
