package repo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitCmd runs a real git command in dir with a fixed identity, failing the test
// on error. Used to set up source repositories the mirror functions act on.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func initSource(t *testing.T) (dir, sha string) {
	t.Helper()
	dir = t.TempDir()
	gitCmd(t, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "first")
	return dir, gitCmd(t, dir, "rev-parse", "HEAD")
}

func TestCheckoutFlow(t *testing.T) {
	ctx := context.Background()
	src, wantSHA := initSource(t)

	mirror := filepath.Join(t.TempDir(), "mirror")
	if err := EnsureMirror(ctx, mirror, src, ""); err != nil {
		t.Fatalf("EnsureMirror: %v", err)
	}
	if err := FetchBranch(ctx, mirror, "main", ""); err != nil {
		t.Fatalf("FetchBranch: %v", err)
	}
	sha, err := ResolveBranchSHA(ctx, mirror, "main")
	if err != nil {
		t.Fatalf("ResolveBranchSHA: %v", err)
	}
	if sha != wantSHA {
		t.Fatalf("resolved sha = %q, want %q", sha, wantSHA)
	}

	scratch := filepath.Join(t.TempDir(), "wt")
	if err := CreateWorktree(ctx, mirror, sha, scratch); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(scratch, "hello.txt"))
	if err != nil || string(got) != "hi" {
		t.Fatalf("worktree content = %q, err %v", got, err)
	}
	if err := RemoveWorktree(ctx, mirror, scratch); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("expected worktree removed, stat err = %v", err)
	}
}

// The explicit refspec must keep refs/heads/<ref> current; a stale ref here would
// be the exact stale-review bug FetchBranch exists to prevent.
func TestFetchUpdatesStaleRef(t *testing.T) {
	ctx := context.Background()
	src, firstSHA := initSource(t)

	mirror := filepath.Join(t.TempDir(), "mirror")
	if err := EnsureMirror(ctx, mirror, src, ""); err != nil {
		t.Fatal(err)
	}
	if err := FetchBranch(ctx, mirror, "main", ""); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("hi again"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, src, "commit", "-am", "second")
	secondSHA := gitCmd(t, src, "rev-parse", "HEAD")
	if secondSHA == firstSHA {
		t.Fatal("setup: second commit did not change HEAD")
	}

	if err := FetchBranch(ctx, mirror, "main", ""); err != nil {
		t.Fatal(err)
	}
	sha, err := ResolveBranchSHA(ctx, mirror, "main")
	if err != nil {
		t.Fatal(err)
	}
	if sha != secondSHA {
		t.Fatalf("after re-fetch sha = %q, want updated %q", sha, secondSHA)
	}
}

func TestEnsureMirrorOriginMismatch(t *testing.T) {
	ctx := context.Background()
	src, _ := initSource(t)

	mirror := filepath.Join(t.TempDir(), "mirror")
	if err := EnsureMirror(ctx, mirror, src, ""); err != nil {
		t.Fatal(err)
	}
	err := EnsureMirror(ctx, mirror, "/some/other/repo", "")
	if err == nil || !strings.Contains(err.Error(), "delete the mirror") {
		t.Fatalf("expected origin-mismatch error, got %v", err)
	}
}
