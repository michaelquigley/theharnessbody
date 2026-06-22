// Package repo wraps the git CLI. It offers two shapes: path-based free
// functions for a read-only checkout pipeline — mirror a remote, fetch a branch,
// resolve its SHA, and create/remove an ephemeral detached worktree (lifted from
// otis) — and a Repo handle bound to one working directory for status, staging,
// commit, pull, and push (lifted from sexton). Both are thin wrappers around the
// `git` command; this package owns no opinion about where mirrors or scratch
// worktrees live — the caller supplies paths.
package repo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Repo is a git handle bound to one working directory.
type Repo struct {
	root       string
	sshCommand string
}

// New returns a Repo for the working directory at root, or nil if root is not a
// git repository. When sshKey is non-empty it is used as the git transport key.
func New(root, sshKey string) *Repo {
	g := &Repo{root: root}
	if !g.IsRepo() {
		return nil
	}
	g.sshCommand = buildSSHCommand(sshKey)
	return g
}

func (g *Repo) IsRepo() bool {
	_, err := g.run("rev-parse", "--git-dir")
	return err == nil
}

func (g *Repo) Status(ctx context.Context) (*Status, error) {
	out, err := g.runCtx(ctx, "status", "--porcelain", "-b", "-uall")
	if err != nil {
		return nil, err
	}
	return parseStatus(out), nil
}

func (g *Repo) StageAll(ctx context.Context) error {
	_, err := g.runCtx(ctx, "add", "-A")
	return err
}

func (g *Repo) Commit(ctx context.Context, message string) error {
	dirty, err := g.IsDirty(ctx)
	if err != nil {
		return err
	}
	if !dirty {
		return ErrNothingToCommit
	}

	_, err = g.runCtx(ctx, "commit", "-m", message)
	return err
}

func (g *Repo) Pull(ctx context.Context, remote, branch string) (pulled bool, err error) {
	dirty, err := g.IsDirty(ctx)
	if err != nil {
		return false, err
	}
	if dirty {
		return false, ErrDirtyWorkingTree
	}

	before, beforeErr := g.HEAD(ctx)
	out, err := g.runCtx(ctx, "pull", "--rebase", remote, branch)
	if err != nil {
		if isConflictOutput(out) {
			return false, ErrConflict
		}
		if isNoRemoteOutput(out) {
			return false, ErrNoRemote
		}
		return false, fmt.Errorf("%w: %s", ErrPullFailed, strings.TrimSpace(out))
	}

	if beforeErr == nil {
		after, err := g.HEAD(ctx)
		if err != nil {
			return false, err
		}
		return before != after, nil
	}

	pulled = !isAlreadyUpToDateOutput(out)
	return pulled, nil
}

func (g *Repo) Push(ctx context.Context, remote, branch string) error {
	out, err := g.runCtx(ctx, "push", remote, "HEAD:"+branch)
	if err != nil {
		if isNoRemoteOutput(out) {
			return ErrNoRemote
		}
		return fmt.Errorf("%w: %s", ErrPushFailed, strings.TrimSpace(out))
	}
	return nil
}

func (g *Repo) RebaseAbort(ctx context.Context) error {
	_, err := g.runCtx(ctx, "rebase", "--abort")
	return err
}

func (g *Repo) Diff() (string, error) {
	return g.run("diff", "HEAD")
}

func (g *Repo) DiffStaged(ctx context.Context) (string, error) {
	return g.runCtx(ctx, "diff", "--staged", "HEAD")
}

func (g *Repo) DiffStat(ctx context.Context) (string, error) {
	return g.runCtx(ctx, "diff", "--stat", "HEAD")
}

func (g *Repo) IsDirty(ctx context.Context) (bool, error) {
	out, err := g.runCtx(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (g *Repo) Branch(ctx context.Context) (string, error) {
	out, err := g.runCtx(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g *Repo) ShortHEAD(ctx context.Context) (string, error) {
	out, err := g.runCtx(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// HEAD returns the full SHA of the current HEAD.
func (g *Repo) HEAD(ctx context.Context) (string, error) {
	out, err := g.runCtx(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CommitTime returns the author timestamp of HEAD.
func (g *Repo) CommitTime(ctx context.Context) (time.Time, error) {
	out, err := g.runCtx(ctx, "log", "-1", "--format=%aI")
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(out))
}

func (g *Repo) run(args ...string) (string, error) {
	return g.runCtx(context.Background(), args...)
}

func (g *Repo) runCtx(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.root
	if g.sshCommand != "" {
		cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+g.sshCommand)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		if stderr.Len() > 0 {
			return stderr.String(), err
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func isConflictOutput(out string) bool {
	return strings.Contains(out, "conflict") || strings.Contains(out, "CONFLICT")
}

func isNoRemoteOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "no remote") ||
		strings.Contains(lower, "no configured push destination") ||
		strings.Contains(lower, "no such remote") ||
		strings.Contains(lower, "does not appear to be a git repository")
}

func isAlreadyUpToDateOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "already up to date") ||
		(strings.Contains(lower, "current branch") && strings.Contains(lower, "is up to date"))
}
