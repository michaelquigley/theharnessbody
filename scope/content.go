package scope

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultPerFileBytes = 8192
	defaultTotalBytes   = 262144
)

// Options bounds how much content BuildContent inlines. Zero values fall back to
// 8 KiB per file and 256 KiB total. The caps apply to file content and diffs
// alike.
type Options struct {
	PerFileBytes    int
	TotalScopeBytes int
}

// Content is the manifest-plus-inline result ready to drop into a prompt.
type Content struct {
	Kind    string
	GitHead string
	Files   []ManifestFile
	Inline  []InlineContent
}

// ManifestFile records one file in scope: its real size, whether it was inlined,
// and a "<orig>-><kept> bytes" note when truncated.
type ManifestFile struct {
	Path      string
	Size      int64
	Truncated string
	Inline    bool
}

// InlineContent is one inlined block: a (possibly truncated) file body, or a diff
// when Diff is true.
type InlineContent struct {
	Path    string
	Content string
	Diff    bool
}

// BuildContent turns a Resolved file set into manifest-plus-inline prompt content,
// bounded by opts. For full/paths scope it inlines budgeted file content; for
// recent scope it inlines budgeted per-file diffs against Resolved.BaseSHA. An
// empty recent scope (no changes in the window) yields empty content, not an error.
func BuildContent(ctx context.Context, worktree string, resolved Resolved, opts Options) (Content, error) {
	if opts.PerFileBytes <= 0 {
		opts.PerFileBytes = defaultPerFileBytes
	}
	if opts.TotalScopeBytes <= 0 {
		opts.TotalScopeBytes = defaultTotalBytes
	}
	head, err := gitLine(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		return Content{}, err
	}
	content := Content{Kind: resolved.Kind, GitHead: head}

	switch resolved.Kind {
	case KindFull, KindPaths:
		items, err := fileItems(resolved.Files, worktree)
		if err != nil {
			return Content{}, err
		}
		return budgetedInline(content, items, opts), nil
	case KindRecent:
		if len(resolved.Files) == 0 {
			return content, nil
		}
		if resolved.BaseSHA == "" {
			return Content{}, fmt.Errorf("base sha is required for recent scope content")
		}
		items, err := diffItems(ctx, resolved.Files, resolved.BaseSHA, worktree)
		if err != nil {
			return Content{}, err
		}
		return budgetedInline(content, items, opts), nil
	default:
		return Content{}, fmt.Errorf("unknown scope kind %q", resolved.Kind)
	}
}

type inlineItem struct {
	path    string
	content string
	diff    bool
}

func fileItems(files []string, worktree string) ([]inlineItem, error) {
	items := make([]inlineItem, 0, len(files))
	for _, relpath := range files {
		raw, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(relpath)))
		if err != nil {
			return nil, err
		}
		items = append(items, inlineItem{path: relpath, content: string(raw)})
	}
	return items, nil
}

func diffItems(ctx context.Context, files []string, baseSHA, worktree string) ([]inlineItem, error) {
	items := make([]inlineItem, 0, len(files))
	for _, relpath := range files {
		raw, err := gitOutput(ctx, worktree, "diff", baseSHA+"..HEAD", "--", relpath)
		if err != nil {
			return nil, err
		}
		items = append(items, inlineItem{path: relpath, content: string(raw), diff: true})
	}
	return items, nil
}

// budgetedInline applies the per-file and total byte caps uniformly to file
// content and diffs, recording a "<orig>-><kept> bytes" note for anything
// truncated and listing every item in the manifest even when the budget is spent.
func budgetedInline(content Content, items []inlineItem, opts Options) Content {
	total := 0
	for _, it := range items {
		raw := it.content
		manifest := ManifestFile{Path: it.path, Size: int64(len(raw))}
		limit := opts.PerFileBytes
		if limit > len(raw) {
			limit = len(raw)
		}
		if total >= opts.TotalScopeBytes {
			manifest.Truncated = fmt.Sprintf("%d->0 bytes", len(raw))
			content.Files = append(content.Files, manifest)
			continue
		}
		remaining := opts.TotalScopeBytes - total
		if limit > remaining {
			limit = remaining
		}
		if limit < len(raw) {
			manifest.Truncated = fmt.Sprintf("%d->%d bytes", len(raw), limit)
		}
		manifest.Inline = limit > 0
		content.Files = append(content.Files, manifest)
		if limit > 0 {
			content.Inline = append(content.Inline, InlineContent{Path: it.path, Content: raw[:limit], Diff: it.diff})
			total += limit
		}
	}
	return content
}
