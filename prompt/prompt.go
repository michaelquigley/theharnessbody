// Package prompt provides the domain-neutral primitives a review prompt is
// assembled from. It deliberately does not offer one universal Build: what a
// code reviewer says (a body-of-knowledge slice, prior findings) and what a
// design reviewer says (verdict definitions, settled decisions) are genuinely
// different, so those domain sections stay in the tools. The body provides the
// load-bearing parts — safe fencing, fenced content blocks, scope-content
// rendering, and the output-schema block — and each tool composes its own prompt
// from them.
package prompt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/michaelquigley/theharnessbody/scope"
)

// Fence returns a backtick fence guaranteed to be longer than any run of
// backticks in content, so arbitrary content (including code that itself contains
// fences) inlines without breaking out. The minimum is three backticks (standard
// markdown). This reconciles otis's and mercurius's near-identical-but-divergent
// fence helpers into one.
func Fence(content string) string {
	longest := 0
	current := 0
	for _, r := range content {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	n := longest + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("`", n)
}

// FencedBlock wraps content in a safe fence (trailing newlines trimmed), ending
// with a newline. Use it to inline any blob — a file, a diff, an artifact.
func FencedBlock(content string) string {
	fence := Fence(content)
	return fence + "\n" + strings.TrimRight(content, "\n") + "\n" + fence + "\n"
}

// ScopeContent renders a scope.Content (manifest plus inline blocks) into prompt
// text: the scope kind and git HEAD, a bulleted file manifest with sizes and
// truncation notes, and each inline block in a safe fence labelled content or
// diff. This is where prompt pairs with the scope package.
func ScopeContent(c scope.Content) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Scope kind: `%s`\n", c.Kind))
	b.WriteString(fmt.Sprintf("Git HEAD: `%s`\n\n", c.GitHead))

	b.WriteString("### File manifest\n\n")
	if len(c.Files) == 0 {
		b.WriteString("(empty scope)\n\n")
	} else {
		for _, f := range c.Files {
			b.WriteString(fmt.Sprintf("- `%s`", f.Path))
			if f.Size > 0 {
				b.WriteString(fmt.Sprintf(" (%d bytes)", f.Size))
			}
			if f.Truncated != "" {
				b.WriteString(fmt.Sprintf(" truncated: %s", f.Truncated))
			}
			if f.Inline {
				b.WriteString(" inline")
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("### Inline content\n\n")
	if len(c.Inline) == 0 {
		b.WriteString("(no inline content)\n\n")
		return b.String()
	}
	for _, in := range c.Inline {
		label := "content"
		if in.Diff {
			label = "diff"
		}
		b.WriteString(fmt.Sprintf("#### %s %s\n\n", label, in.Path))
		b.WriteString(FencedBlock(in.Content))
		b.WriteString("\n")
	}
	return b.String()
}

// SchemaBlock renders the standard output-schema instruction: the "single JSON
// object only" directive followed by the pretty-printed schema in a json fence.
// The schema is the tool's (for example reviewer/schema's Findings or Verdict).
func SchemaBlock(schemaDoc json.RawMessage) string {
	pretty := prettyJSON(schemaDoc)
	// Use a dynamic fence: a tool's schema can carry descriptions or examples
	// containing backticks, which would break a fixed ``` fence.
	fence := Fence(pretty)

	var b strings.Builder
	b.WriteString("Respond with a single JSON object only. No prose before or after, no markdown fence around the object, no commentary. Your response must conform exactly to this schema:\n\n")
	b.WriteString(fence)
	b.WriteString("json\n")
	b.WriteString(pretty)
	b.WriteString("\n")
	b.WriteString(fence)
	b.WriteString("\n")
	return b.String()
}

func prettyJSON(raw json.RawMessage) string {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}
