package record

import (
	"fmt"
	"strings"
	"time"
)

type SynopsisEntry struct {
	SessionID            string
	OpenedAt             time.Time
	ClosedAt             time.Time
	Reviewer             SynopsisReviewer
	ReviewContextPresent bool
	ReviewFocusPresent   bool
	LastError            *SynopsisError
	Rounds               []SynopsisRound
}

type SynopsisReviewer struct {
	Name  string
	Impl  string
	Model string
}

type SynopsisError struct {
	Code    string
	Message string
}

type SynopsisRound struct {
	Number          int
	OpenedAt        time.Time
	LogPath         string
	NotesPath       string
	HasNotes        bool
	Verdict         string
	ReviewerName    string
	ReviewerSummary string
	Unparseable     bool
	Decisions       []Decision
	Commentary      string
	Sections        []Section
}

func WriteSynopsis(path string, entry SynopsisEntry) error {
	var b strings.Builder

	writeSynopsisFrontmatter(&b, entry)
	b.WriteString("# Session synopsis\n\n")
	writeSynopsisSummary(&b, entry)
	writeSynopsisRoundOutcomes(&b, entry)
	writeSynopsisRoundDetail(&b, entry)

	return writeAtomic(path, []byte(b.String()))
}

func writeSynopsisFrontmatter(b *strings.Builder, entry SynopsisEntry) {
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("session_id: %s\n", entry.SessionID))
	b.WriteString(fmt.Sprintf("opened_at: %s\n", entry.OpenedAt.UTC().Format(time.RFC3339)))
	if !entry.ClosedAt.IsZero() {
		b.WriteString(fmt.Sprintf("closed_at: %s\n", entry.ClosedAt.UTC().Format(time.RFC3339)))
	}
	b.WriteString(fmt.Sprintf("round_count: %d\n", len(entry.Rounds)))
	b.WriteString("reviewer:\n")
	b.WriteString(fmt.Sprintf("  name: %s\n", entry.Reviewer.Name))
	if entry.Reviewer.Impl != "" {
		b.WriteString(fmt.Sprintf("  impl: %s\n", entry.Reviewer.Impl))
	}
	if entry.Reviewer.Model != "" {
		b.WriteString(fmt.Sprintf("  model: %s\n", entry.Reviewer.Model))
	}
	b.WriteString(fmt.Sprintf("review_context_present: %t\n", entry.ReviewContextPresent))
	b.WriteString(fmt.Sprintf("review_focus_present: %t\n", entry.ReviewFocusPresent))
	if entry.LastError != nil {
		b.WriteString("last_error:\n")
		b.WriteString(fmt.Sprintf("  code: %s\n", entry.LastError.Code))
		b.WriteString(fmt.Sprintf("  message: %s\n", entry.LastError.Message))
	}
	b.WriteString("---\n\n")
}

func writeSynopsisSummary(b *strings.Builder, entry SynopsisEntry) {
	if len(entry.Rounds) == 0 {
		b.WriteString("## Summary\n\nSession closed with no rounds.\n\n")
		return
	}
	latest := entry.Rounds[len(entry.Rounds)-1].Verdict
	if latest == "" {
		latest = "unknown"
	}
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("Closed %d round(s) with reviewer '%s'. Latest verdict: '%s'.\n\n", len(entry.Rounds), entry.Reviewer.Name, latest))
}

func writeSynopsisRoundOutcomes(b *strings.Builder, entry SynopsisEntry) {
	if len(entry.Rounds) == 0 {
		return
	}
	b.WriteString("## Round outcomes\n\n")
	b.WriteString("| round | opened_at | verdict | notes |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, round := range entry.Rounds {
		notes := "-"
		if round.HasNotes {
			notes = "yes"
		}
		verdict := round.Verdict
		if verdict == "" {
			verdict = "-"
		}
		b.WriteString(fmt.Sprintf("| %02d | %s | %s | %s |\n",
			round.Number,
			round.OpenedAt.UTC().Format(time.RFC3339),
			escapeTableCell(verdict),
			notes,
		))
	}
	b.WriteString("\n")
}

func writeSynopsisRoundDetail(b *strings.Builder, entry SynopsisEntry) {
	if len(entry.Rounds) == 0 {
		return
	}
	b.WriteString("## Round detail\n\n")
	for _, round := range entry.Rounds {
		b.WriteString(fmt.Sprintf("### Round %02d\n\n", round.Number))
		b.WriteString(fmt.Sprintf("- Opened: %s\n", round.OpenedAt.UTC().Format(time.RFC3339)))
		if round.Verdict != "" {
			b.WriteString(fmt.Sprintf("- Verdict: %s\n", round.Verdict))
		}
		if round.LogPath != "" {
			b.WriteString(fmt.Sprintf("- Log: %s\n", round.LogPath))
		}
		if round.HasNotes && round.NotesPath != "" {
			b.WriteString(fmt.Sprintf("- Notes: %s\n", round.NotesPath))
		}
		b.WriteString("\n")
		if round.Unparseable {
			b.WriteString("_reviewer output unparseable; see the round log for raw JSON_\n\n")
			continue
		}
		if strings.TrimSpace(round.ReviewerSummary) != "" {
			b.WriteString("**Reviewer summary:** ")
			b.WriteString(strings.TrimSpace(round.ReviewerSummary))
			b.WriteString("\n\n")
		}
		for _, section := range round.Sections {
			if strings.TrimSpace(section.Heading) == "" {
				continue
			}
			b.WriteString("**")
			b.WriteString(section.Heading)
			b.WriteString("**\n\n")
			b.WriteString(strings.TrimRight(section.Markdown, "\n"))
			b.WriteString("\n\n")
		}
		if len(round.Decisions) > 0 {
			b.WriteString("**Decisions**\n\n")
			for _, decision := range round.Decisions {
				b.WriteString(fmt.Sprintf("- **%s** (ref: %s): %s.\n", decision.Disposition, decision.Ref, strings.TrimRight(decision.Note, ".")))
			}
			b.WriteString("\n")
		}
		if strings.TrimSpace(round.Commentary) != "" {
			b.WriteString("**Commentary**\n\n")
			for _, line := range strings.Split(strings.TrimRight(round.Commentary, "\n"), "\n") {
				b.WriteString("> ")
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
}
