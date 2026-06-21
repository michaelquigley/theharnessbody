package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

// The reference schemas the body ships must both live inside the envelope.
func TestGuardEnvelopeAcceptsReferenceSchemas(t *testing.T) {
	cases := []struct {
		name string
		doc  json.RawMessage
	}{
		{"verdict", Verdict()},
		{"findings", Findings()},
	}
	for _, tc := range cases {
		if err := GuardEnvelope(tc.doc); err != nil {
			t.Fatalf("GuardEnvelope(%s) = %v, want nil", tc.name, err)
		}
	}
}

// otis's findings put a nested object (location) inside the array's items — the
// construct that broke codex. The guard must catch it and name it.
const otisNestedObjectSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["findings"],
  "properties": {
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["severity", "title", "location"],
        "properties": {
          "severity": {"type": "string", "enum": ["low", "medium", "high"]},
          "title": {"type": "string"},
          "location": {
            "type": "object",
            "additionalProperties": false,
            "required": ["file", "lines"],
            "properties": {
              "file": {"type": "string"},
              "lines": {"type": "string"}
            }
          }
        }
      }
    }
  }
}`

// otis's other nested construct: a bok_refs array inside the array's items.
const otisNestedArraySchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["findings"],
  "properties": {
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["severity", "title", "bok_refs"],
        "properties": {
          "severity": {"type": "string", "enum": ["low", "medium", "high"]},
          "title": {"type": "string"},
          "bok_refs": {"type": "array", "items": {"type": "string"}}
        }
      }
    }
  }
}`

func TestGuardEnvelopeRejectsNestedObjectInArrayItem(t *testing.T) {
	err := GuardEnvelope(json.RawMessage(otisNestedObjectSchema))
	if err == nil {
		t.Fatal("expected envelope violation for a nested object inside an array item, got nil")
	}
	if !strings.Contains(err.Error(), "location") {
		t.Fatalf("error should name the offending property: %v", err)
	}
}

func TestGuardEnvelopeRejectsNestedArrayInArrayItem(t *testing.T) {
	err := GuardEnvelope(json.RawMessage(otisNestedArraySchema))
	if err == nil {
		t.Fatal("expected envelope violation for a nested array inside an array item, got nil")
	}
	if !strings.Contains(err.Error(), "bok_refs") {
		t.Fatalf("error should name the offending property: %v", err)
	}
}

// A $ref into $defs would bypass the nested-construct check (the array item is a
// {"$ref": ...}, which checkArrayItem reads as a scalar) — so the nested object
// hiding in the def slips through. The guard rejects $ref outright to close it.
const refBypassSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["findings"],
  "$defs": {
    "finding": {
      "type": "object",
      "additionalProperties": false,
      "required": ["location"],
      "properties": {
        "location": {
          "type": "object",
          "additionalProperties": false,
          "required": ["file"],
          "properties": {"file": {"type": "string"}}
        }
      }
    }
  },
  "properties": {
    "findings": {"type": "array", "items": {"$ref": "#/$defs/finding"}}
  }
}`

func TestGuardEnvelopeRejectsRef(t *testing.T) {
	err := GuardEnvelope(json.RawMessage(refBypassSchema))
	if err == nil {
		t.Fatal("expected $ref to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "$ref") {
		t.Fatalf("error should name $ref: %v", err)
	}
}

func TestValidateFindings(t *testing.T) {
	valid := json.RawMessage(`{"summary":"ok","findings":[{"id":"f1","severity":"major","file":"a.go","lines":"10-12","claim":"x","rationale":"y","suggestion":null}]}`)
	if err := Validate(valid, Findings()); err != nil {
		t.Fatalf("valid findings output rejected: %v", err)
	}

	missingField := json.RawMessage(`{"summary":"ok","findings":[{"id":"f1","severity":"major"}]}`)
	if err := Validate(missingField, Findings()); err == nil {
		t.Fatal("expected schema violation for a finding missing required fields")
	}

	badSeverity := json.RawMessage(`{"summary":"ok","findings":[{"id":"f1","severity":"nope","file":"a.go","lines":"1","claim":"x","rationale":"y","suggestion":null}]}`)
	if err := Validate(badSeverity, Findings()); err == nil {
		t.Fatal("expected schema violation for an out-of-enum severity")
	}
}

// The generalized validator must keep validating mercurius's verdict output
// unchanged — the abstraction-is-right check.
func TestValidateVerdict(t *testing.T) {
	valid := json.RawMessage(`{"verdict":"ready_to_build","summary":"s","concerns":[],"questions":[],"advisory_notes":[]}`)
	if err := Validate(valid, Verdict()); err != nil {
		t.Fatalf("valid verdict output rejected: %v", err)
	}
}
