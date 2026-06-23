// Package reviewer defines the backend-agnostic reviewer interface the body's
// review tooling is built on. A reviewer turns a pre-assembled prompt plus a
// supplied output schema into raw structured output. The reviewer is
// schema-agnostic — it never hard-codes an output shape — and it does not
// validate its own output: Raw comes back unvalidated and the caller checks it
// against the request schema (see reviewer/schema).
package reviewer

import (
	"context"
	"encoding/json"
)

// Reviewer runs one structured review against some backend (codex, claude, pi, ...).
type Reviewer interface {
	Review(ctx context.Context, req ReviewRequest) (ReviewResponse, error)
}

// ReviewRequest is the payload handed to a reviewer.
type ReviewRequest struct {
	// Prompt is the fully assembled prompt; the caller inlines any artifact
	// content it wants the backend to see.
	Prompt string
	// Schema is the output schema passed to the backend. The caller validates the
	// response against this same schema.
	Schema json.RawMessage
	// WorkingDir is the directory the backend process runs in. It should be a
	// neutral, caller-controlled directory — NOT the untrusted checkout under
	// review. Some backends (codex, claude) auto-load a directory's AGENTS.md /
	// CLAUDE.md as instructions, so running inside the reviewed code would let it
	// prompt-inject the reviewer. Deliver the code to review through Prompt (see
	// the prompt and scope packages), not through WorkingDir.
	WorkingDir string
}

// ReviewResponse carries the reviewer's raw structured output and diagnostics.
// Raw is unvalidated; the caller validates it against the request schema.
type ReviewResponse struct {
	Raw        json.RawMessage
	UsageNotes string
}
