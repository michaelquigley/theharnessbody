# theharnessbody — current state

A Go library: the reusable substrate that review harnesses are built on. This
file describes what exists, verifiable against the code. Forward-looking intent
lives in `../future/`.

Today the body carries one subsystem.

## Reviewer subsystem (`reviewer/`)

A backend-agnostic reviewer interface and the output-schema tooling around it.

- **`reviewer`** — the `Reviewer` interface and `ReviewRequest`/`ReviewResponse`.
  A reviewer turns a pre-assembled prompt plus a supplied output schema into raw
  structured output. It is schema-agnostic and does not validate its own output:
  `Raw` comes back unvalidated and the caller checks it against the request
  schema.
- **`reviewer/schema`** — schema-agnostic validation (`Validate`, `Compile`)
  against a supplied JSON Schema; two reference schemas, `Verdict()` (mercurius's
  verbatim) and `Findings()` (the code-review shape); and `GuardEnvelope`, which
  rejects the schema construct that broke otis's codex reviewer. See
  `reviewer-output-contract.md`.
- **`reviewer/jsonout`** — extracts a single JSON object from messy reviewer
  output (bare, markdown-fenced, or embedded in prose). Stdlib only.
- **`reviewer/codex`, `reviewer/claude`, `reviewer/pi`** — backend adapters,
  lifted from mercurius's proven implementations. The codex adapter additionally
  carries otis's per-reviewer `env` merge and makes `config.toml` optional.
- **`reviewer/dummy`** — a fixed-response reviewer for model-free tests of the
  plumbing.
- **`reviewer/confirm`** — a build-tagged (`integration`) live confirmation that
  the findings schema passes each backend. Run with
  `go test -tags integration -v -timeout 15m ./reviewer/confirm/`; each backend
  skips if its binary is absent.

The reviewer-output contract — the codex envelope, the findings shape, and the
guard that enforces it — is in `reviewer-output-contract.md`.
