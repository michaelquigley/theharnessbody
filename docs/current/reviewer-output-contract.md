# Reviewer Output Contract

A reviewer in the body turns a prompt and a supplied output schema into one JSON
object. The body validates that object against the schema the caller handed in,
and it owns no canonical output types of its own — the schema belongs to the
tool, not the body. What the body does own is a piece of hard-won knowledge about
which schemas a backend will actually accept, and a guard that enforces it before
a schema ever reaches a model.

## The codex envelope

The knowledge came out of otis. Its codex reviewer kept drawing a 400 from the
model API — `invalid_json_schema` — and the diagnosis at the time was strictness:
OpenAI's structured-output mode is fussy, so the schema must put every property in
`required`, optional fields must be nullable, and constraint keywords like
`minLength`, `maxItems`, even `$schema` were assumed to be the trouble. otis
stripped all of it. The 400 persisted anyway and was never root-caused; the
working conclusion was that the whole `codex exec --output-schema` approach might
be the wrong shape.

But mercurius runs that exact approach in heavy production use, and it works. It
writes its schema to codex verbatim — `$schema`, `minLength`, even a `maxItems` it
adds at runtime, none of it stripped — and codex accepts it. So strictness was a
red herring. The two schemas differ structurally, and that is where the real
boundary sits: otis's findings put a nested object (`location: {file, lines}`) and
a nested array (`bok_refs: [string]`) inside the items of an array. mercurius
never nests anything inside an array item. That — nesting inside array items — is
the construct codex rejects, not any of the keywords otis spent its effort on.

Stated positively, the envelope is the shape mercurius has proven codex accepts:

- a root object, `additionalProperties: false`, every property in `required`;
- arrays of objects whose item fields are scalars, enums, or nullable scalar
  unions (`["string", "null"]`);
- `$schema`, `minLength`, `maxItems` and their kind are tolerated — they are not
  the problem.

And the one thing to avoid: **no nested object or nested array inside an array's
items.** Flatten it. A `location: {file, lines}` becomes sibling `file` and
`lines` strings; a `bok_refs` array moves out of the item entirely.

This is confirmed empirically, not just by inference: the body's findings schema
(below) was run live through codex, through claude's native `--json-schema`, and
through pi, and all three returned valid output. (See `reviewer/confirm`, build
tag `integration`.)

## The findings shape

The body ships two reference schemas. `schema.Verdict()` is mercurius's
verdict/concerns schema, verbatim — the proven reference. `schema.Findings()` is
the shape for code review, designed from the start to live inside the envelope:

```
output:
  summary:  string                       one-paragraph high-level read
  findings: [ finding ]
finding:
  id:         string                     always present; the harness reconciles new vs resurfaced
  severity:   enum [blocker, major, minor]
  file:       string                     flat — no nested location object
  lines:      string                     "42" / "42-58" / "N/A"
  claim:      string                     what is wrong
  rationale:  string                     why it matters
  suggestion: string | null              concrete fix, or null
```

It carries mercurius's discipline where that pays. `claim` and `rationale` are
separate — what is wrong, and why it matters. `suggestion` is nullable, for
findings with no clean fix. `severity` is `blocker`/`major`/`minor`, the scale
that encodes whether a finding blocks the merge, which is the judgment a code
review is actually making. `location` is flat, `file` and `lines` as sibling
strings — both because that is cleaner and because it keeps the shape inside the
envelope. A tool that wants more — a body-of-knowledge reference, say — adds it as
its own extension; the core finding stays portable.

## The guard

`schema.GuardEnvelope` walks a schema and rejects exactly the proven-bad
construct: an object that sits as the items of an array may not carry a property
that is itself an object or an array, and an array's items may not themselves be
an array. It is deliberately narrow — it encodes a measured lesson, not a taste,
and it does not reach for a broader shape it has no evidence for. It does assume
inline schemas and rejects `$ref` outright: it cannot resolve a reference, so a
`$ref` array item would otherwise slip the check, and codex's structured output is
itself unproven with refs. A tool authoring a new output schema runs it through
the guard at author time and learns, before a single model call, whether the
schema will survive codex.

The regression test that locks this in is the otis schema itself: the guard
rejects otis's nested `location` and `bok_refs` (and any `$ref`), and accepts both
reference schemas.

## The schema-agnostic boundary

The body validates against a supplied schema and owns no canonical output types.
`Verdict()` and `Findings()` are starting points, not a tax — mercurius brings its
verdict schema, a code-review tool brings its findings schema, and the body's job
is only to validate against whichever was handed in and to guard it against the
envelope. The reviewer never sees the schema's meaning: it passes the schema to
the backend, gets raw output back, and hands that to the caller to validate.
