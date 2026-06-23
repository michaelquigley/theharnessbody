// Package scope selects which files a review should look at and packs their
// content into a prompt within a byte budget. Resolve picks the files (every
// tracked file, an explicit list, or what changed in a recent window);
// BuildContent reads them into a bounded manifest-plus-inline form, using diffs
// for recent scope. Both run read-only git against a worktree path (for example
// one produced by the repo package); scope owns no config types — the caller
// supplies plain inputs.
package scope

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Scope kinds.
const (
	KindFull   = "full"
	KindPaths  = "paths"
	KindRecent = "recent"
)

// emptyTreeSHA is git's well-known empty-tree object, used as the diff base when
// the oldest recent commit has no parent (the initial commit).
const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Spec describes how to select files. Type is one of KindFull/KindPaths/KindRecent.
type Spec struct {
	Type   string
	Paths  []string      // for KindPaths
	Window time.Duration // for KindRecent
}

// Resolved is the selected file set. BaseSHA is set for recent scope — the commit
// the changed files are diffed against.
type Resolved struct {
	Kind    string
	Files   []string
	BaseSHA string
}

// Resolve selects the files for spec against the worktree. now is used only for
// recent scope and defaults to the current time when zero.
func Resolve(ctx context.Context, worktree string, spec Spec, now time.Time) (Resolved, error) {
	switch spec.Type {
	case KindFull:
		files, err := gitListFiles(ctx, worktree)
		return Resolved{Kind: KindFull, Files: files}, err
	case KindPaths:
		files, err := resolvePathScope(ctx, worktree, spec.Paths)
		return Resolved{Kind: KindPaths, Files: files}, err
	case KindRecent:
		files, base, err := resolveRecentScope(ctx, worktree, spec.Window, now)
		return Resolved{Kind: KindRecent, Files: files, BaseSHA: base}, err
	default:
		return Resolved{}, fmt.Errorf("unknown scope type %q", spec.Type)
	}
}

func resolvePathScope(ctx context.Context, worktree string, entries []string) ([]string, error) {
	tracked, err := trackedSet(ctx, worktree)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, entry := range entries {
		entry = filepath.ToSlash(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if escapesWorktree(entry) {
			return nil, fmt.Errorf("path %q is outside the worktree", entry)
		}
		abs := filepath.Join(worktree, filepath.FromSlash(entry))
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			files, err := gitListFiles(ctx, worktree, entry)
			if err != nil {
				return nil, err
			}
			addFiles(seen, files)
			continue
		}
		if containsGlobMeta(entry) {
			matches, err := filepath.Glob(abs)
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				rel, err := filepath.Rel(worktree, match)
				if err != nil {
					return nil, err
				}
				rel = filepath.ToSlash(rel)
				if rel == ".." || strings.HasPrefix(rel, "../") {
					continue // a glob that escaped the worktree
				}
				info, err := os.Stat(match)
				if err != nil {
					return nil, err
				}
				if info.IsDir() {
					files, err := gitListFiles(ctx, worktree, rel)
					if err != nil {
						return nil, err
					}
					addFiles(seen, files)
				} else if _, ok := tracked[rel]; ok {
					seen[rel] = struct{}{} // only tracked files from a glob
				}
			}
			continue
		}
		if info, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("%s: %w", entry, err)
		} else if info.IsDir() {
			files, err := gitListFiles(ctx, worktree, entry)
			if err != nil {
				return nil, err
			}
			addFiles(seen, files)
		} else if _, ok := tracked[entry]; ok {
			seen[entry] = struct{}{}
		} else {
			return nil, fmt.Errorf("%s: not a tracked file", entry)
		}
	}
	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}

// escapesWorktree reports whether a path entry would resolve outside the worktree
// (an absolute path, or one that climbs out via ..). Path scope stays inside
// tracked repo content; a caller wanting broader access opts in elsewhere.
func escapesWorktree(entry string) bool {
	if filepath.IsAbs(entry) {
		return true
	}
	clean := filepath.Clean(entry)
	return clean == ".." || strings.HasPrefix(clean, "../")
}

// trackedSet is the set of git-tracked files in the worktree, used to keep path
// scope from reaching untracked or out-of-tree files.
func trackedSet(ctx context.Context, worktree string) (map[string]struct{}, error) {
	files, err := gitListFiles(ctx, worktree)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(files))
	for _, f := range files {
		set[f] = struct{}{}
	}
	return set, nil
}

func resolveRecentScope(ctx context.Context, worktree string, window time.Duration, now time.Time) ([]string, string, error) {
	if window <= 0 {
		return nil, "", fmt.Errorf("recent scope requires a positive window")
	}
	if now.IsZero() {
		now = time.Now()
	}
	since := now.UTC().Add(-window).Format(time.RFC3339)
	commits, err := gitLines(ctx, worktree, "log", "--first-parent", "--since="+since, "--pretty=%H", "HEAD")
	if err != nil {
		return nil, "", err
	}
	if len(commits) == 0 {
		return nil, "", nil
	}
	oldest := commits[len(commits)-1]
	base, err := gitLine(ctx, worktree, "rev-parse", oldest+"^1")
	if err != nil {
		base = emptyTreeSHA
	}
	files, err := gitLines(ctx, worktree, "diff", "--name-only", base+"..HEAD")
	if err != nil {
		return nil, "", err
	}
	return files, base, nil
}

func gitListFiles(ctx context.Context, worktree string, paths ...string) ([]string, error) {
	args := []string{"ls-files"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	files, err := gitLines(ctx, worktree, args...)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func gitLine(ctx context.Context, worktree string, args ...string) (string, error) {
	lines, err := gitLines(ctx, worktree, args...)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("git %s produced no output", strings.Join(args, " "))
	}
	return lines[0], nil
}

func gitLines(ctx context.Context, worktree string, args ...string) ([]string, error) {
	raw, err := gitOutput(ctx, worktree, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(filepath.ToSlash(line))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func gitOutput(ctx context.Context, worktree string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", worktree}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w%s", strings.Join(args, " "), err, stderrSuffix(stderr.Bytes()))
	}
	return raw, nil
}

func stderrSuffix(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	return ": " + string(raw)
}

func containsGlobMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func addFiles(seen map[string]struct{}, files []string) {
	for _, file := range files {
		seen[file] = struct{}{}
	}
}
