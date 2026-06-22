package scope

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func baseEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
}

func datedEnv(rfc3339 string) []string {
	return append(baseEnv(), "GIT_AUTHOR_DATE="+rfc3339, "GIT_COMMITTER_DATE="+rfc3339)
}

func git(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, baseEnv(), "init", "-b", "main")
	return dir
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func findManifest(t *testing.T, c Content, path string) ManifestFile {
	t.Helper()
	for _, f := range c.Files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("no manifest entry for %q", path)
	return ManifestFile{}
}

func findInline(t *testing.T, c Content, path string) InlineContent {
	t.Helper()
	for _, i := range c.Inline {
		if i.Path == path {
			return i
		}
	}
	t.Fatalf("no inline entry for %q", path)
	return InlineContent{}
}

func TestResolveFull(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "sub/b.txt", "b")
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, baseEnv(), "commit", "-m", "init")

	r, err := Resolve(context.Background(), dir, Spec{Type: KindFull}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != KindFull || !equal(r.Files, []string{"a.txt", "sub/b.txt"}) {
		t.Fatalf("resolved = %+v", r)
	}
}

func TestResolvePathsFileDirGlob(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "sub/b.txt", "b")
	writeFile(t, dir, "sub/c.go", "c")
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, baseEnv(), "commit", "-m", "init")

	// a single file plus a whole directory
	r, err := Resolve(context.Background(), dir, Spec{Type: KindPaths, Paths: []string{"a.txt", "sub"}}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !equal(r.Files, []string{"a.txt", "sub/b.txt", "sub/c.go"}) {
		t.Fatalf("paths(file+dir) = %v", r.Files)
	}

	// a glob
	r, err = Resolve(context.Background(), dir, Spec{Type: KindPaths, Paths: []string{"sub/*.go"}}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !equal(r.Files, []string{"sub/c.go"}) {
		t.Fatalf("paths(glob) = %v", r.Files)
	}
}

func TestResolveRecent(t *testing.T) {
	dir := initRepo(t)
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	writeFile(t, dir, "old.txt", "old")
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, datedEnv("2026-06-17T12:00:00Z"), "commit", "-m", "old")
	oldSHA := git(t, dir, baseEnv(), "rev-parse", "HEAD")

	writeFile(t, dir, "new.txt", "new")
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, datedEnv("2026-06-20T11:00:00Z"), "commit", "-m", "new")

	r, err := Resolve(context.Background(), dir, Spec{Type: KindRecent, Window: 24 * time.Hour}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(r.Files, []string{"new.txt"}) {
		t.Fatalf("recent files = %v, want [new.txt]", r.Files)
	}
	if r.BaseSHA != oldSHA {
		t.Fatalf("base = %q, want old %q", r.BaseSHA, oldSHA)
	}
}

func TestBuildContentTruncatesPerFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "big.txt", strings.Repeat("x", 100))
	writeFile(t, dir, "small.txt", "tiny")
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, baseEnv(), "commit", "-m", "init")

	r, _ := Resolve(context.Background(), dir, Spec{Type: KindFull}, time.Time{})
	c, err := BuildContent(context.Background(), dir, r, Options{PerFileBytes: 10, TotalScopeBytes: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if c.GitHead == "" {
		t.Fatal("expected GitHead set")
	}

	big := findManifest(t, c, "big.txt")
	if big.Size != 100 || big.Truncated != "100->10 bytes" || !big.Inline {
		t.Fatalf("big.txt manifest = %+v", big)
	}
	if got := findInline(t, c, "big.txt"); len(got.Content) != 10 {
		t.Fatalf("big.txt inline len = %d, want 10", len(got.Content))
	}

	small := findManifest(t, c, "small.txt")
	if small.Truncated != "" || !small.Inline {
		t.Fatalf("small.txt should be inlined untruncated: %+v", small)
	}
}

func TestBuildContentTotalBudgetExhausted(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", strings.Repeat("x", 100))
	writeFile(t, dir, "b.txt", strings.Repeat("y", 100))
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, baseEnv(), "commit", "-m", "init")

	r, _ := Resolve(context.Background(), dir, Spec{Type: KindFull}, time.Time{})
	c, err := BuildContent(context.Background(), dir, r, Options{PerFileBytes: 50, TotalScopeBytes: 50})
	if err != nil {
		t.Fatal(err)
	}

	// a.txt sorts first: gets the whole 50-byte budget (truncated from 100).
	a := findManifest(t, c, "a.txt")
	if a.Truncated != "100->50 bytes" || !a.Inline {
		t.Fatalf("a.txt manifest = %+v", a)
	}
	// b.txt: budget exhausted, listed in the manifest but not inlined.
	b := findManifest(t, c, "b.txt")
	if b.Truncated != "100->0 bytes" || b.Inline {
		t.Fatalf("b.txt manifest = %+v", b)
	}
}

func TestBuildContentRecentInlinesDiffs(t *testing.T) {
	dir := initRepo(t)
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	writeFile(t, dir, "f.txt", "line1\n")
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, datedEnv("2026-06-17T12:00:00Z"), "commit", "-m", "old")

	writeFile(t, dir, "f.txt", "line1\nline2\n")
	git(t, dir, baseEnv(), "add", "-A")
	git(t, dir, datedEnv("2026-06-20T11:00:00Z"), "commit", "-m", "new")

	r, err := Resolve(context.Background(), dir, Spec{Type: KindRecent, Window: 24 * time.Hour}, now)
	if err != nil {
		t.Fatal(err)
	}
	c, err := BuildContent(context.Background(), dir, r, Options{})
	if err != nil {
		t.Fatal(err)
	}
	in := findInline(t, c, "f.txt")
	if !in.Diff {
		t.Fatal("recent scope inline should be a diff")
	}
	if !strings.Contains(in.Content, "+line2") {
		t.Fatalf("diff should show the added line, got:\n%s", in.Content)
	}
}
