# CHANGELOG

## Unreleased

FEATURE: Initial reviewer subsystem — a backend-agnostic `Reviewer` interface with `codex`, `claude`, `pi`, and `dummy` adapters (lifted from mercurius's proven implementations; the codex adapter adds otis's per-reviewer `env` merge and an optional `config.toml`), schema-agnostic output validation, and a `GuardEnvelope` check that rejects the schema construct that broke otis's codex reviewer — nested objects or arrays inside array items. Ships two reference schemas: mercurius's verdict/concerns shape and a new code-review findings shape, confirmed live against codex and claude.

CHANGE: Stood up the project's agent-facing scaffolding — `AGENTS.md` pointing back at the grimoire substrate, the `docs/current` + `docs/future` + `docs/journal` structure, the reviewer-output contract, and this changelog.
