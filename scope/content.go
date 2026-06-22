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
// 8 KiB per file and 256 KiB total.
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
// bounded by opts. For full/paths scope it inlines (truncated) file content; for
// recent scope it inlines per-file diffs against Resolved.BaseSHA.
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
		return buildFileInlineContent(content, resolved.Files, worktree, opts)
	case KindRecent:
		if resolved.BaseSHA == "" {
			return Content{}, fmt.Errorf("base sha is required for recent scope content")
		}
		return buildRecentDiffContent(ctx, content, resolved.Files, resolved.BaseSHA, worktree)
	default:
		return Content{}, fmt.Errorf("unknown scope kind %q", resolved.Kind)
	}
}

func buildFileInlineContent(content Content, files []string, worktree string, opts Options) (Content, error) {
	total := 0
	for _, relpath := range files {
		path := filepath.Join(worktree, filepath.FromSlash(relpath))
		raw, err := os.ReadFile(path)
		if err != nil {
			return Content{}, err
		}
		manifest := ManifestFile{
			Path: relpath,
			Size: int64(len(raw)),
		}
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
			content.Inline = append(content.Inline, InlineContent{
				Path:    relpath,
				Content: string(raw[:limit]),
			})
			total += limit
		}
	}
	return content, nil
}

func buildRecentDiffContent(ctx context.Context, content Content, files []string, baseSHA, worktree string) (Content, error) {
	for _, relpath := range files {
		raw, err := gitOutput(ctx, worktree, "diff", baseSHA+"..HEAD", "--", relpath)
		if err != nil {
			return Content{}, err
		}
		diff := string(raw)
		content.Files = append(content.Files, ManifestFile{
			Path:   relpath,
			Size:   int64(len(diff)),
			Inline: true,
		})
		content.Inline = append(content.Inline, InlineContent{
			Path:    relpath,
			Content: diff,
			Diff:    true,
		})
	}
	return content, nil
}
