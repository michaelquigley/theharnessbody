//go:build integration

// Package confirm holds the live, build-tagged confirmation that the body's
// findings schema actually passes each reviewer backend — the loop-closer for the
// codex/schema de-risk. Run with:
//
//	go test -tags integration -v -timeout 15m ./reviewer/confirm/
//
// Each backend skips if its binary is not on PATH. The point is empirical: the
// findings schema is safe by construction (it stays inside mercurius's proven
// envelope), and these runs confirm it on the real backends, where each may have
// its own structured-output quirks.
package confirm

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/michaelquigley/theharnessbody/reviewer"
	"github.com/michaelquigley/theharnessbody/reviewer/claude"
	"github.com/michaelquigley/theharnessbody/reviewer/codex"
	"github.com/michaelquigley/theharnessbody/reviewer/pi"
	"github.com/michaelquigley/theharnessbody/reviewer/schema"
)

const smokeInstruction = `You are running a smoke test of a structured-output pipeline. Do not review any code or inspect the working directory. Return exactly one JSON object that conforms to the supplied output schema: set "summary" to "smoke test ok" and "findings" to an empty array. Output only the JSON object and nothing else.`

func smokePrompt() string {
	return smokeInstruction + "\n\nOutput schema:\n```json\n" + string(schema.Findings()) + "\n```\n"
}

func confirmBackend(t *testing.T, binary string, r reviewer.Reviewer) {
	t.Helper()
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("%s not on PATH: %v", binary, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dir := t.TempDir()
	if out, err := exec.CommandContext(ctx, "git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init working dir: %v\n%s", err, out)
	}

	resp, err := r.Review(ctx, reviewer.ReviewRequest{
		Prompt:     smokePrompt(),
		Schema:     schema.Findings(),
		WorkingDir: dir,
	})
	if err != nil {
		// A backend that is installed but not logged in is an environment gap, not
		// a defect in the body — skip it, the same as an absent binary.
		if isAuthError(err) {
			t.Skipf("%s present but not authenticated on this host: %v", binary, err)
		}
		t.Fatalf("%s review failed: %v", binary, err)
	}
	if err := schema.Validate(resp.Raw, schema.Findings()); err != nil {
		t.Fatalf("%s output failed findings-schema validation: %v\nraw: %s", binary, err, resp.Raw)
	}
	t.Logf("%s OK\nusage: %s\nraw: %s", binary, resp.UsageNotes, resp.Raw)
}

func isAuthError(err error) bool {
	s := err.Error()
	for _, marker := range []string{"No API key", "Use /login", "not logged in", "authentication"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

func TestConfirmCodex(t *testing.T) {
	confirmBackend(t, "codex", codex.New(codex.Options{Model: os.Getenv("THB_CODEX_MODEL")}))
}

func TestConfirmClaude(t *testing.T) {
	confirmBackend(t, "claude", claude.New(claude.Options{Model: os.Getenv("THB_CLAUDE_MODEL")}))
}

func TestConfirmPi(t *testing.T) {
	confirmBackend(t, "pi", pi.New(pi.Options{Model: os.Getenv("THB_PI_MODEL")}))
}
