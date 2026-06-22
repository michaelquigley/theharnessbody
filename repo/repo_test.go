package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installFakeGit(t *testing.T, logPath string, script string) {
	t.Helper()
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake git: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_LOG", logPath)
}

func readGitLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read git log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func TestPullUsesExplicitRemoteAndBranch(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "%s\n" "$*" >> "$GIT_LOG"
case "$*" in
  "status --porcelain") exit 0 ;;
  "rev-parse HEAD") echo "abc123"; exit 0 ;;
  "pull --rebase origin main") echo "Already up to date."; exit 0 ;;
  *) echo "unexpected args: $*" >&2; exit 99 ;;
esac
`)
	g := &Repo{root: t.TempDir()}
	pulled, err := g.Pull(context.Background(), "origin", "main")
	if err != nil {
		t.Fatalf("expected pull to succeed, got %v", err)
	}
	if pulled {
		t.Fatal("expected already-up-to-date pull to report no changes")
	}
	logLines := readGitLog(t, gitLog)
	want := []string{"status --porcelain", "rev-parse HEAD", "pull --rebase origin main", "rev-parse HEAD"}
	if len(logLines) != len(want) {
		t.Fatalf("expected %d git invocations, got %d: %v", len(want), len(logLines), logLines)
	}
	for i := range want {
		if logLines[i] != want[i] {
			t.Fatalf("invocation %d = %q, want %q", i, logLines[i], want[i])
		}
	}
}

func TestPullReportsChangesWhenHeadChanges(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "%s\n" "$*" >> "$GIT_LOG"
case "$*" in
  "status --porcelain") exit 0 ;;
  "rev-parse HEAD")
    count="$(grep -c "rev-parse HEAD" "$GIT_LOG")"
    if [ "$count" -eq 1 ]; then echo "abc123"; else echo "def456"; fi
    exit 0 ;;
  "pull --rebase origin main") echo "Fast-forward"; exit 0 ;;
  *) echo "unexpected args: $*" >&2; exit 99 ;;
esac
`)
	g := &Repo{root: t.TempDir()}
	pulled, err := g.Pull(context.Background(), "origin", "main")
	if err != nil {
		t.Fatalf("expected pull to succeed, got %v", err)
	}
	if !pulled {
		t.Fatal("expected changed HEAD to report pulled changes")
	}
}

func TestPushUsesExplicitRemoteAndBranch(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "%s\n" "$*" >> "$GIT_LOG"
case "$*" in
  "push origin HEAD:main") exit 0 ;;
  *) echo "unexpected args: $*" >&2; exit 99 ;;
esac
`)
	g := &Repo{root: t.TempDir()}
	if err := g.Push(context.Background(), "origin", "main"); err != nil {
		t.Fatalf("expected push to succeed, got %v", err)
	}
	logLines := readGitLog(t, gitLog)
	if len(logLines) != 1 || logLines[0] != "push origin HEAD:main" {
		t.Fatalf("unexpected push invocations: %v", logLines)
	}
}

func TestPullUnknownRemoteReturnsErrNoRemote(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "%s\n" "$*" >> "$GIT_LOG"
case "$*" in
  "status --porcelain") exit 0 ;;
  "rev-parse HEAD") echo "abc123"; exit 0 ;;
  "pull --rebase origin main") echo "fatal: 'origin' does not appear to be a git repository" >&2; exit 1 ;;
  *) echo "unexpected args: $*" >&2; exit 99 ;;
esac
`)
	g := &Repo{root: t.TempDir()}
	if _, err := g.Pull(context.Background(), "origin", "main"); !errors.Is(err, ErrNoRemote) {
		t.Fatalf("expected ErrNoRemote, got %v", err)
	}
}

func TestPushFailureSurfacesGitOutput(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "%s\n" "$*" >> "$GIT_LOG"
case "$*" in
  "push origin HEAD:main") echo "error: src refspec HEAD does not match any" >&2; exit 1 ;;
  *) echo "unexpected args: $*" >&2; exit 99 ;;
esac
`)
	g := &Repo{root: t.TempDir()}
	err := g.Push(context.Background(), "origin", "main")
	if !errors.Is(err, ErrPushFailed) {
		t.Fatalf("expected ErrPushFailed, got %v", err)
	}
	if !strings.Contains(err.Error(), "does not match any") {
		t.Fatalf("expected git output in error, got %v", err)
	}
}

func TestCommitNothingToCommit(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "%s\n" "$*" >> "$GIT_LOG"
case "$*" in
  "status --porcelain") exit 0 ;;
  *) echo "unexpected args: $*" >&2; exit 99 ;;
esac
`)
	g := &Repo{root: t.TempDir()}
	if err := g.Commit(context.Background(), "msg"); !errors.Is(err, ErrNothingToCommit) {
		t.Fatalf("expected ErrNothingToCommit, got %v", err)
	}
}

func TestBuildSSHCommand(t *testing.T) {
	tests := []struct{ name, key, want string }{
		{"plain", "/home/u/.ssh/id", "ssh -i '/home/u/.ssh/id' -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"},
		{"spaces", "/home/u/my keys/id", "ssh -i '/home/u/my keys/id' -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"},
		{"embedded quote", "/home/u/o'brien/id", `ssh -i '/home/u/o'\''brien/id' -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new`},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildSSHCommand(tt.key); got != tt.want {
				t.Fatalf("buildSSHCommand(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestRunCtxInjectsSSHCommand(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "GIT_SSH_COMMAND=%s\n" "$GIT_SSH_COMMAND" >> "$GIT_LOG"
echo "main"
exit 0
`)
	g := &Repo{root: t.TempDir(), sshCommand: "ssh -i '/keys/deploy' -o IdentitiesOnly=yes"}
	if _, err := g.Branch(context.Background()); err != nil {
		t.Fatalf("Branch() error = %v", err)
	}
	lines := readGitLog(t, gitLog)
	want := "GIT_SSH_COMMAND=ssh -i '/keys/deploy' -o IdentitiesOnly=yes"
	if len(lines) == 0 || lines[0] != want {
		t.Fatalf("git environment = %v, want first line %q", lines, want)
	}
}

func TestRunCtxWithoutSSHKeyInheritsAmbient(t *testing.T) {
	gitLog := filepath.Join(t.TempDir(), "git.log")
	t.Setenv("GIT_SSH_COMMAND", "ambient-ssh")
	installFakeGit(t, gitLog, `#!/bin/sh
printf "GIT_SSH_COMMAND=%s\n" "$GIT_SSH_COMMAND" >> "$GIT_LOG"
echo "main"
exit 0
`)
	g := &Repo{root: t.TempDir()}
	if _, err := g.Branch(context.Background()); err != nil {
		t.Fatalf("Branch() error = %v", err)
	}
	lines := readGitLog(t, gitLog)
	want := "GIT_SSH_COMMAND=ambient-ssh"
	if len(lines) == 0 || lines[0] != want {
		t.Fatalf("git environment = %v, want first line %q", lines, want)
	}
}
