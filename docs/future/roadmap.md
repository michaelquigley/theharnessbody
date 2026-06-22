# Roadmap

Intent, not built state. The body grows by harvesting proven parts from the constellation (`mercurius`, `sexton`, `otis`) into reusable subsystems, then building a code-review tool on top, then refactoring the donors onto the body.

The arc:

1. **Reviewer subsystem** — codex, claude, pi, dummy, schema-agnostic validation, the envelope guard. *Built* (see `../current/`).
2. **Mattermost + command** — a `mattermost.Client` (REST + WebSocket + @mention/trigger command extraction + allowed-users) and a transport-agnostic `command.Registry`, from sexton. *Built*. (otis's message-size truncation deferred per YAGNI until terminus posts large findings.)
3. **Git (`repo/`)** — path-based mirror + fetch + resolve + ephemeral worktree (from otis, read-only checkout) and a `Repo` working-tree handle for status/stage/commit/pull/push (from sexton). *Built*.
4. **Remaining subsystems** — harvest the collection `terminus` needs: `scope/` (scope kinds + byte budgeting, from otis), `prompt/` (assembly primitives), `config/`, `record/` (round log + atomic writes, from mercurius), `errs/`, `cli/`.
5. **terminus** (working name for the "new otis") — the first tool built *on* the body, composed from these parts. Brings its own findings schema (already shaped here) and its body-of-knowledge handling as a tool-side extension.
6. **Refactor mercurius and sexton** onto the body — last. mercurius is the correctness check throughout (its verdict schema must keep validating unchanged through the body's helper); it is not refactored until the end.

This deliberately makes terminus the first consumer rather than refactoring mercurius/sexton early as an extraction forcing-function; the donor refactor is deferred to the end.

## Open design decisions (carried forward)

- **BoK: core, flavor, or tool-side?** The working boundary rule is "the body is what the three tools duplicate." A body-of-knowledge is used by exactly one tool, and `otis-bok` is already its own repo — so the current lean is that BoK stays tool-side, a `terminus` extension, not a body package. Decide before building the prompt subsystem.
- **One shared output envelope, or two domain schemas?** Settled for now as two (verdict, findings), with the body schema-agnostic. Worth revisiting whether a minimal shared finding envelope is worth standardizing so the notification/record layers can render any tool's output generically.
- **Prompt: assembly primitives vs. one parameterized builder?** Leaning primitives, each tool composing its own prompt — decide when `prompt/` is harvested.
