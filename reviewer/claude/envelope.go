package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/michaelquigley/theharnessbody/reviewer/jsonout"
)

// errNotEnvelope marks output that could not be read as a claude '--output-format
// json' envelope at all (empty or non-json). it is distinct from an envelope that
// parsed but reported an error or carried no json object, which produce their own
// descriptive errors.
var errNotEnvelope = errors.New("claude reviewer output is not a json envelope")

// envelope is the subset of the claude '--output-format json' result object that
// the body reads. with '--json-schema', the validated object lands in
// 'structured_output'; 'result' holds the model's text answer or an error message.
type envelope struct {
	IsError          bool            `json:"is_error"`
	Subtype          string          `json:"subtype"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	NumTurns         int             `json:"num_turns"`
	DurationMS       int             `json:"duration_ms"`
	SessionID        string          `json:"session_id"`
}

// parseEnvelope reads the claude json envelope from stdout and returns the review
// object. it prefers the schema-validated 'structured_output' and falls back to
// extracting a json object from the 'result' text. it does not validate the object
// against the review schema - the caller does that.
func parseEnvelope(stdout []byte) (json.RawMessage, envelope, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, envelope{}, fmt.Errorf("%w: empty output", errNotEnvelope)
	}

	var env envelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil, envelope{}, fmt.Errorf("%w: %s", errNotEnvelope, snippet(trimmed))
	}

	if env.IsError {
		msg := strings.TrimSpace(env.Result)
		if msg == "" {
			msg = "(no result message)"
		}
		if env.Subtype != "" {
			return nil, env, fmt.Errorf("claude reviewer error (subtype='%s'): %s", env.Subtype, msg)
		}
		return nil, env, fmt.Errorf("claude reviewer error: %s", msg)
	}

	// structured_output is the native schema-enforced field; when present and
	// non-null it is authoritative. If we can't extract a json object from it, fail
	// rather than silently falling back to the freeform result text, which could be
	// a different or wrong object. Fall back to result only when structured_output
	// is absent or null. (jsonout.Object also guards against it being a json string
	// or array rather than an object.)
	if so := bytes.TrimSpace(env.StructuredOutput); len(so) > 0 && !bytes.Equal(so, []byte("null")) {
		raw, err := jsonout.Object(so)
		if err != nil {
			return nil, env, fmt.Errorf("claude reviewer structured_output is present but not a json object: %w", err)
		}
		return raw, env, nil
	}

	raw, err := jsonout.Object([]byte(env.Result))
	if err != nil {
		return nil, env, fmt.Errorf("claude reviewer returned no json object in result: %w", err)
	}
	return raw, env, nil
}

func snippet(b []byte) string {
	const max = 200
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
