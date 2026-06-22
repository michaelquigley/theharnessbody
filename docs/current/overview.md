# theharnessbody — current state

A Go library: the reusable substrate that review harnesses are built on. This file describes what exists, verifiable against the code. Forward-looking intent lives in `../future/`.

Today the body carries four subsystems.

## Reviewer subsystem (`reviewer/`)

A backend-agnostic reviewer interface and the output-schema tooling around it.

- **`reviewer`** — the `Reviewer` interface and `ReviewRequest`/`ReviewResponse`. A reviewer turns a pre-assembled prompt plus a supplied output schema into raw structured output. It is schema-agnostic and does not validate its own output: `Raw` comes back unvalidated and the caller checks it against the request schema.
- **`reviewer/schema`** — schema-agnostic validation (`Validate`, `Compile`) against a supplied JSON Schema; two reference schemas, `Verdict()` (mercurius's verbatim) and `Findings()` (the code-review shape); and `GuardEnvelope`, which rejects the schema construct that broke otis's codex reviewer. See `reviewer-output-contract.md`.
- **`reviewer/jsonout`** — extracts a single JSON object from messy reviewer output (bare, markdown-fenced, or embedded in prose). Stdlib only.
- **`reviewer/codex`, `reviewer/claude`, `reviewer/pi`** — backend adapters, lifted from mercurius's proven implementations. The codex adapter additionally carries otis's per-reviewer `env` merge and makes `config.toml` optional.
- **`reviewer/dummy`** — a fixed-response reviewer for model-free tests of the plumbing.
- **`reviewer/confirm`** — a build-tagged (`integration`) live confirmation that the findings schema passes each backend. Run with `go test -tags integration -v -timeout 15m ./reviewer/confirm/`; each backend skips if its binary is absent.

The reviewer-output contract — the codex envelope, the findings shape, and the guard that enforces it — is in `reviewer-output-contract.md`.

## Mattermost + command (`mattermost/`, `command/`)

Chat ingress/egress and a transport-agnostic command dispatcher, lifted and generalized from sexton.

- **`command`** — a `Registry` of named commands (`Register(name, summary, Handler)`), with `Dispatch(ctx, text) string` (help on empty/`help`, the handler's reply or a rendered error, or an unknown-command message) and `Help()`. Transport-agnostic: the same registry serves a chat bot and a CLI.
- **`mattermost`** — a `Client` that posts via REST (`PostMessage`) and listens over WebSocket (with reconnect), resolving the bot identity, extracting commands from @mentions or configured trigger words, and filtering by allowed users. On a matched message it calls a `Responder func(ctx, command) string` and posts a non-empty reply to the originating channel. `Registry.Dispatch` satisfies `Responder`, so wiring is one line: `mc.Start(reg.Dispatch)`.
- **`mattermost` (integration)** — build-tagged live smoke tests (`TestConfirmPostMessage`, `TestConfirmConnect`), gated on `THB_MM_*` env.

`mattermost` doesn't import `command` and vice versa; the app wires them.

## Git (`repo/`)

Thin `git`-CLI wrappers in two shapes, lifted from otis and sexton.

- **Read-only checkout (free functions, from otis)** — `EnsureMirror` (bare `--mirror` clone, origin-verified), `FetchBranch` (explicit refspec so the ref never goes stale), `ResolveBranchSHA`, and `CreateWorktree`/`RemoveWorktree`/`PruneWorktrees`/`CaptureHEAD` for an ephemeral detached checkout. This is how a reviewer gets a branch's code onto disk without touching any working copy. The package is unopinionated about paths — the caller says where mirrors and scratch worktrees live.
- **Working tree (a `Repo` handle, from sexton)** — `New(dir, sshKey)` then `Status`/`IsDirty`/`Branch`/`HEAD`/`ShortHEAD`/`CommitTime`/`Diff*` and the write ops `StageAll`/`Commit`/`Pull` (`--rebase`)/`Push`/`RebaseAbort`, with sentinel errors (`ErrNothingToCommit`, `ErrConflict`, `ErrNoRemote`, …).
- Shared: one SSH-command builder (`-o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new`, shell-quoted key).

## Scope (`scope/`)

Selects which files a review looks at and packs them into a prompt within a byte budget, from otis. Runs read-only git against a worktree (for example one the `repo` package produced); owns no config types.

- **`Resolve(ctx, worktree, Spec, now)`** picks files three ways: `KindFull` (every tracked file), `KindPaths` (an explicit list of files, directories, or globs), `KindRecent` (files changed within `Spec.Window`, with the diff base commit recorded). Returns `Resolved{Kind, Files, BaseSHA}`.
- **`BuildContent(ctx, worktree, Resolved, Options)`** reads the selection into a `Content` manifest plus inline blocks, bounded by per-file and total byte caps (default 8 KiB / 256 KiB) with a `<orig>-><kept> bytes` truncation note. Recent scope inlines per-file diffs instead of whole files.
