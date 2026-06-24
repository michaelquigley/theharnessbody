// Package record writes durable broker review records. It generalizes
// mercurius's round log writer without importing any broker-specific schema:
// callers supply reviewer output, artifact manifests, and optional rendered
// markdown sections for domain-specific audit material.
package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArtifactManifestEntry records one artifact snapshot in a review record.
type ArtifactManifestEntry struct {
	Name         string
	SourcePath   string
	SnapshotPath string
	Hash         string
	Size         int64
}

// ReviewerOutput records one reviewer's schema-valid output.
type ReviewerOutput struct {
	Name       string
	Raw        json.RawMessage
	UsageNotes string
}

// Section is an optional caller-rendered markdown section appended after the
// reviewer output. record stays domain-neutral; the caller owns the section body.
type Section struct {
	Heading  string
	Markdown string
}

// Decision records one human decision attached to a review record.
type Decision struct {
	Ref         string
	Disposition string
	Note        string
}

// Entry contains the immutable content of one review record.
type Entry struct {
	SessionID   string
	RoundNumber int
	OpenedAt    time.Time
	Verdict     string
	PromptPath  string
	Manifest    []ArtifactManifestEntry
	Reviewers   []ReviewerOutput
	Sections    []Section
}

// WriteInitial writes the immutable review record atomically.
func WriteInitial(path string, entry Entry) error {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("session_id: %s\n", entry.SessionID))
	b.WriteString(fmt.Sprintf("round_number: %d\n", entry.RoundNumber))
	b.WriteString(fmt.Sprintf("opened_at: %s\n", entry.OpenedAt.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("verdict: %s\n", entry.Verdict))
	if entry.PromptPath != "" {
		b.WriteString(fmt.Sprintf("prompt_path: %s\n", entry.PromptPath))
	}
	b.WriteString("reviewers:\n")
	for _, reviewer := range entry.Reviewers {
		b.WriteString(fmt.Sprintf("  - %s\n", reviewer.Name))
	}
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# Round %02d\n\n", entry.RoundNumber))
	b.WriteString("## Artifact manifest\n\n")
	b.WriteString("| name | source_path | snapshot_path | size | hash |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, artifact := range entry.Manifest {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %s |\n",
			escapeTableCell(artifact.Name),
			escapeTableCell(artifact.SourcePath),
			escapeTableCell(artifact.SnapshotPath),
			artifact.Size,
			escapeTableCell(artifact.Hash),
		))
	}
	b.WriteString("\n")

	b.WriteString("## Reviewer outputs\n\n")
	for _, reviewer := range entry.Reviewers {
		b.WriteString(fmt.Sprintf("### %s\n\n", reviewer.Name))
		b.WriteString(fmt.Sprintf("**Usage notes:** `%s`\n\n", escapeBackticks(reviewer.UsageNotes)))
		b.WriteString("```json\n")
		b.WriteString(prettyJSON(reviewer.Raw))
		b.WriteString("\n```\n\n")
	}

	for _, section := range entry.Sections {
		heading := strings.TrimSpace(section.Heading)
		if heading == "" {
			continue
		}
		b.WriteString("## ")
		b.WriteString(heading)
		b.WriteString("\n\n")
		b.WriteString(strings.TrimRight(section.Markdown, "\n"))
		b.WriteString("\n\n")
	}

	return writeAtomic(path, []byte(b.String()))
}

// WriteNotes writes a sibling notes file containing commentary and decisions.
func WriteNotes(path string, commentary string, decisions []Decision) error {
	var b strings.Builder

	b.WriteString("# Notes\n\n")
	b.WriteString("## Commentary\n\n")
	if strings.TrimSpace(commentary) == "" {
		b.WriteString("_no commentary recorded_\n\n")
	} else {
		b.WriteString(strings.TrimRight(commentary, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString("## Decisions\n\n")
	if len(decisions) == 0 {
		b.WriteString("_no decisions recorded_\n")
	} else {
		for _, decision := range decisions {
			b.WriteString(fmt.Sprintf("- **%s** (ref: %s): %s.\n", decision.Disposition, decision.Ref, strings.TrimRight(decision.Note, ".")))
		}
	}
	return writeAtomic(path, []byte(b.String()))
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".record-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

func escapeBackticks(s string) string {
	return strings.ReplaceAll(s, "`", "'")
}

func prettyJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		return string(raw)
	}
	return b.String()
}
