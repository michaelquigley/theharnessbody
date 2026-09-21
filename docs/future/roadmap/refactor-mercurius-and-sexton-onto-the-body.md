---
title: refactor mercurius and sexton onto the body
state: inbox
created: 2026-09-21
tags: [epic]
log:
  - stamp: 2026-09-21
    note: migrated from the legacy docs/future/roadmap.md (step 8 of the harvest arc); the coupled pi-adapter copy is recorded in docs/journal/2026-08-17.md
---

Move the two donor tools onto theharnessbody, replacing their internal copies of the harvested subsystems (reviewer backends, the mattermost client and command registry, the git wrappers) with imports of this module. mercurius is the correctness check throughout: its verdict schema must keep validating unchanged through the body's helper, so it is refactored last.

## why

This was deliberately the final step of the harvest arc: terminus was made the first consumer rather than using an early donor refactor as the extraction forcing-function. terminus now ships on the body; mercurius and sexton still carry their originals, and at least one of those copies (`mercurius/internal/reviewer/pi`) has to be changed in lockstep with this repo until the refactor lands.
