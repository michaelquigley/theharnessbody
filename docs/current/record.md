# Record

`record/` writes durable review records for broker-shaped tools. It is the domain-neutral harvest of mercurius's round log writer: the package knows about review records, artifact manifests, reviewer raw JSON, notes, and synopsis files, but it does not know about mercurius verdict schemas, terminus qualities, or any tool's finding model.

`WriteInitial(path, Entry)` writes the immutable record atomically by creating a temporary file in the target directory, chmodding it to `0600`, and renaming it into place. The rendered document contains YAML frontmatter, an artifact manifest table, each reviewer output as pretty JSON, and optional caller-rendered sections. Those sections are the extension point: terminus uses them for selected qualities and classified findings, while mercurius can leave them empty and keep its current round-log shape.

`WriteNotes(path, commentary, decisions)` writes the mutable human-decision companion file. `WriteSynopsis(path, Entry)` provides a generic session-level summary shape for brokers that keep multi-round sessions. Terminus v1 does not use notes or synopses because it is stateless per invocation; they are present so the mercurius refactor can land on the same package without pulling its writer back out of the body.
