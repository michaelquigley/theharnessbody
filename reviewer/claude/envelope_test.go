package claude

import (
	"strings"
	"testing"
)

func TestParseEnvelopeStructuredOutputAuthoritative(t *testing.T) {
	stdout := []byte(`{"structured_output": {"verdict": "ok"}, "result": "ignore me"}`)
	raw, _, err := parseEnvelope(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"verdict"`) {
		t.Fatalf("expected structured_output object, got %s", raw)
	}
	if strings.Contains(string(raw), "ignore me") {
		t.Fatal("used result instead of structured_output")
	}
}

// Present-but-invalid structured_output must fail closed, not fall back to result.
func TestParseEnvelopeStructuredOutputPresentButInvalidFails(t *testing.T) {
	stdout := []byte(`{"structured_output": [1,2,3], "result": "{\"verdict\":\"fallback\"}"}`)
	_, _, err := parseEnvelope(stdout)
	if err == nil {
		t.Fatal("expected failure when structured_output is present but not an object")
	}
	if !strings.Contains(err.Error(), "structured_output") {
		t.Fatalf("error should name structured_output: %v", err)
	}
}

func TestParseEnvelopeNullStructuredOutputFallsBack(t *testing.T) {
	stdout := []byte(`{"structured_output": null, "result": "{\"verdict\":\"from_result\"}"}`)
	raw, _, err := parseEnvelope(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "from_result") {
		t.Fatalf("null structured_output should fall back to result, got %s", raw)
	}
}

func TestParseEnvelopeAbsentStructuredOutputFallsBack(t *testing.T) {
	stdout := []byte(`{"result": "here it is: {\"verdict\":\"from_result\"}"}`)
	raw, _, err := parseEnvelope(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "from_result") {
		t.Fatalf("absent structured_output should fall back to result, got %s", raw)
	}
}

func TestParseEnvelopeIsError(t *testing.T) {
	stdout := []byte(`{"is_error": true, "subtype": "timeout", "result": "boom"}`)
	if _, _, err := parseEnvelope(stdout); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected is_error to surface, got %v", err)
	}
}
