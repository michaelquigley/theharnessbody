# theharnessbody

theharnessbody is the foundational substrate harnesses are built on — the proven,
reusable body of review-domain tooling (reviewer backends, output-schema
validation, and the repo/scope/notification/command plumbing to follow),
separated from any one tool's orchestration. It is what `mercurius`, `sexton`, and
a forthcoming code-review tool (`terminus`, the "new otis") share, harvested from
wherever each capability already exists in its best form. Go module
`github.com/michaelquigley/theharnessbody`.

## Orienting here

The conventions you work under live in the grimoire, not in this file — read them
there rather than inferring from the tree. Via the `grimoire` MCP server, start at
`AGENTS.md` (the grimoire root) and follow the orientation cascade. For work here
the load-bearing notes are `agents/agent-roles` and `agents/design-build-pipeline`
(how the practice runs its agents) and `meta/writing-like-michael` (voice).

## Operating mode: one blended agent

This work is greenfield and runs as a single blended agent rather than the full
four-phase pipeline — the mode the pipeline names for work that doesn't need
formal phase separation. The posture is design-primary, with planning and
implementation folded in. The disciplines that still hold: forward-looking intent
lands in `docs/future/`, built behavior is written into `docs/current/` as it
lands, and consequential or hard-to-reverse decisions get surfaced before they
commit.

The governing instinct, learned from otis: **harvest the proven core, defer the
harness.** Otis stalled because it built ceremony — server, scheduler, state
machine — around a reviewer core that had never run against a model. Don't repeat
that, here or in the tools built on this.

## Conventions

Design docs split current/future: `docs/current/` describes the system as built
(verifiable against the code); `docs/future/` holds intent. See the grimoire's
`meta/where-design-lives`.

Go: use `github.com/michaelquigley/df/dl` for logging and
`github.com/michaelquigley/df/dd` for YAML/JSON marshaling.

Changelog: `CHANGELOG.md` follows the in-house format — newest-first, prose
entries tagged `FEATURE`/`CHANGE`/`FIX`, written into the `## Unreleased` slot.
Full spec: grimoire `software/conventions/changelog-convention`.

## Project memory

Durable knowledge about this project lives in `docs/journal/`, dated files
`docs/journal/YYYY-MM-DD.md`. This is project memory; it does not go in
harness-local storage (`.claude/` or equivalent), where it's invisible to every
other harness and collaborator and dies with the host. Concretely: do not write
to your harness's memory directory or memory tool for this project — even when
the harness presents it as the default place for durable knowledge. That tool is
the silo this convention exists to replace; the journal is the only durable home.

On arrival, read the most recent entries to pick up where the last session left
off, before you start changing things. Treat them as prior-session context, not
verified truth — if an entry conflicts with the code or a `docs/current/` doc,
the code wins.

Write the smallest entry that carries the session's durable insight, and nothing
more. The test for every line: *would a competent agent get this wrong, or waste
time rediscovering it, working from the tree alone?* If it's recoverable by
reading the code, the diff, `docs/current/`, or git history, leave it out.

That filter keeps four kinds of thing and discards the rest:

- **Decisions whose rationale isn't visible in the result** — why a value was
  chosen, what a line guards against, why something that looks like dead code or
  a no-op is load-bearing.
- **Deliberate non-actions** — a change you considered and chose not to make, so
  the next agent doesn't "fix" it. An unchanged file leaves no trace in a diff.
- **Couplings that span files** — two places that must move together, an ordering
  that matters, an assumption one file makes about another.
- **Live state** — what's unverified, unfinished, or waiting on something
  external.

Skip change inventories, restatements of the diff, and play-by-play of how you
worked. There's no write-time approval gate; Michael reviews on commit. Append
to the day's file if it exists, and write the few lines you'd want the next agent
to read — honest and self-contained.
