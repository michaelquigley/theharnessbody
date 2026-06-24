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
	// WorkingDir is the directory the backend process runs in. Most design-review
	// callers use a neutral scratch directory and deliver artifacts through
	// Prompt. Code-review callers may deliberately run in the checkout under
	// review so the backend can read connected source files directly; those
	// callers must use read-only backend modes and account for backend-specific
	// context-file loading. pi suppresses project context files; codex/claude
	// require prompt-level precedence instructions until their adapters grow a
	// review-specific suppression path.
	WorkingDir string
}

// ReviewResponse carries the reviewer's raw structured output and diagnostics.
// Raw is unvalidated; the caller validates it against the request schema.
type ReviewResponse struct {
	Raw        json.RawMessage
	UsageNotes string
}
