# Ownership until v1.0.0

| | |
|---|---|
| Owner | Claude, acting for epistemer (delegated on 2026-11-07) |
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

## What still comes to you first

1. **Releases.** Every tag, from `v0.1.0` to `v1.0.0`. The Go module proxy
   keeps a published version forever.
2. **Your decisions and preferences.** Changes to D1–D11, to preferences you've
   stated, or to anything a developer writes or gets back in a way that isn't a
   clarification.
3. **Anything outside this repository.** zp (M10), staging and production, other
   repositories, GitHub settings, accounts, tokens and paid services.
4. **Speaking for you.** Replies to other people's issues and PRs,
   announcements, posts.
5. **Destructive or irreversible operations.** Force pushes, history rewrites,
   deleting branches that hold unmerged work, deleting releases or the
   repository.
6. **Decisions already reserved for you.** The `cfgx init` skeleton (M8.1) and
   which zp settings become mutable (M10.3). Claude brings a recommendation.

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

- **v1.0.0 needs you at M10.** Porting zp touches your production code and your
  staging environment, so that milestone runs with you, not just for you.
- **Quotas.** agy uses your Gemini quota, and Claude's sessions have context
  limits. Large fan-outs check `agy_status` first.
- **Safety checks still apply.** The auto-mode safety classifier blocks some git
  operations, such as deleting remote branches. Claude leaves those to you
  rather than working around them.
