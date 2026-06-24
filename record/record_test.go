package record

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteInitial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "round.md")
	err := WriteInitial(path, Entry{
		SessionID:   "review-1",
		RoundNumber: 1,
		OpenedAt:    time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
		Verdict:     "clean",
		PromptPath:  "_prompt.md",
		Manifest: []ArtifactManifestEntry{{
			Name:         "spec",
			SourcePath:   "/tmp/spec.md",
			SnapshotPath: "spec.md",
			Size:         12,
			Hash:         "abc",
		}},
		Reviewers: []ReviewerOutput{{
			Name:       "dummy",
			Raw:        json.RawMessage(`{"summary":"ok"}`),
			UsageNotes: "dummy reviewer",
		}},
		Sections: []Section{{Heading: "Selected qualities", Markdown: "| id |\n| --- |\n| df |\n"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"session_id: review-1",
		"verdict: clean",
		"## Artifact manifest",
		"### dummy",
		"## Selected qualities",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("record did not contain %q\n%s", want, text)
		}
	}
}

func TestWriteNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := WriteNotes(path, "handled", []Decision{{Ref: "f1", Disposition: "fixed", Note: "done"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "**fixed** (ref: f1): done.") {
		t.Fatalf("unexpected notes:\n%s", string(raw))
	}
}
