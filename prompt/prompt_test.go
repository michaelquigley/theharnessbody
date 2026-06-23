package prompt

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/michaelquigley/theharnessbody/scope"
)

func TestFence(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{"plain", "hello world", "```"},
		{"empty", "", "```"},
		{"contains-triple", "before ``` after", "````"},
		{"contains-quad", "x ```` y", "`````"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Fence(tc.content); got != tc.want {
				t.Fatalf("Fence(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}

func TestFencedBlock(t *testing.T) {
	if got := FencedBlock("line1\nline2\n"); got != "```\nline1\nline2\n```\n" {
		t.Fatalf("plain FencedBlock = %q", got)
	}
	// content with an embedded fence must be wrapped in a longer fence
	got := FencedBlock("has ``` inside")
	if !strings.HasPrefix(got, "````\n") || !strings.HasSuffix(got, "\n````\n") {
		t.Fatalf("embedded-fence FencedBlock = %q", got)
	}
}

func TestScopeContent(t *testing.T) {
	c := scope.Content{
		Kind:    scope.KindRecent,
		GitHead: "abc123",
		Files: []scope.ManifestFile{
			{Path: "a.go", Size: 100, Truncated: "100->10 bytes", Inline: true},
			{Path: "b.go", Size: 5, Inline: true},
		},
		Inline: []scope.InlineContent{
			{Path: "a.go", Content: "@@ -1 +1 @@\n-old\n+new", Diff: true},
			{Path: "b.go", Content: "package b", Diff: false},
		},
	}
	out := ScopeContent(c)

	for _, want := range []string{
		"Scope kind: `recent`",
		"Git HEAD: `abc123`",
		"- `a.go` (100 bytes) truncated: 100->10 bytes inline",
		"- `b.go` (5 bytes) inline",
		"#### diff a.go",
		"#### content b.go",
		"+new",
		"package b",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("ScopeContent missing %q in:\n%s", want, out)
		}
	}
}

func TestScopeContentEmpty(t *testing.T) {
	out := ScopeContent(scope.Content{Kind: scope.KindFull, GitHead: "h"})
	if !strings.Contains(out, "(empty scope)") || !strings.Contains(out, "(no inline content)") {
		t.Fatalf("empty ScopeContent = %s", out)
	}
}

func TestSchemaBlock(t *testing.T) {
	out := SchemaBlock(json.RawMessage(`{"type":"object","required":["x"]}`))
	if !strings.Contains(out, "single JSON object") {
		t.Fatalf("missing instruction: %s", out)
	}
	if !strings.Contains(out, "```json") {
		t.Fatalf("missing json fence: %s", out)
	}
	// pretty-printed (indented) schema
	if !strings.Contains(out, "\"type\": \"object\"") {
		t.Fatalf("schema not pretty-printed: %s", out)
	}
}

func TestSchemaBlockDynamicFence(t *testing.T) {
	// a schema whose description contains a triple-backtick run must be fenced
	// with a longer fence so it can't break out
	schema := json.RawMessage("{\"type\":\"object\",\"description\":\"wrap in ``` fences\"}")
	out := SchemaBlock(schema)
	if !strings.Contains(out, "````json") {
		t.Fatalf("expected a 4-backtick fence around backtick-containing schema, got:\n%s", out)
	}
	if strings.Count(out, "````") < 2 {
		t.Fatalf("expected matching 4-backtick fences, got:\n%s", out)
	}
	if strings.Contains(out, "```json") && !strings.Contains(out, "````json") {
		t.Fatalf("fixed 3-backtick fence leaked through:\n%s", out)
	}
}
