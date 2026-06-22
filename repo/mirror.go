package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureMirror creates the bare mirror on first use and otherwise verifies the
// existing mirror still points at the configured repo. A repointed repo must not
// silently keep fetching the old origin, so an origin mismatch fails with an
// instruction to delete the mirror rather than re-cloning automatically. The
// mirror is pure, re-creatable cache.
func EnsureMirror(ctx context.Context, mirrorDir, repoURL, sshKey string) error {
	info, err := os.Stat(mirrorDir)
	switch {
	case err == nil && info.IsDir():
		out, gitErr := runGit(ctx, sshKey, "-C", mirrorDir, "remote", "get-url", "origin")
		if gitErr != nil {
			return fmt.Errorf("verify mirror origin for %s: %w", mirrorDir, gitErr)
		}
		got := strings.TrimSpace(string(out))
		if got != repoURL {
			return fmt.Errorf("mirror %s points at origin %q, not configured repo %q; delete the mirror to re-clone", mirrorDir, got, repoURL)
		}
		return nil
	case err == nil:
		return fmt.Errorf("mirror path %s exists but is not a directory", mirrorDir)
	case !os.IsNotExist(err):
		return err
	}
	if err := os.MkdirAll(filepath.Dir(mirrorDir), 0o700); err != nil {
		return err
	}
	if _, err := runGit(ctx, sshKey, "clone", "--mirror", repoURL, mirrorDir); err != nil {
		return fmt.Errorf("clone mirror %s: %w", repoURL, err)
	}
	return nil
}

// FetchBranch updates one branch ref in the mirror. The explicit destination
// refspec is load-bearing: a bare `fetch origin <ref>` updates only FETCH_HEAD
// and leaves refs/heads/<ref> stale, so a later ResolveBranchSHA would resolve
// the old SHA and silently review stale code — the exact failure this guards.
func FetchBranch(ctx context.Context, mirrorDir, ref, sshKey string) error {
	refspec := fmt.Sprintf("+refs/heads/%s:refs/heads/%s", ref, ref)
	if _, err := runGit(ctx, sshKey, "-C", mirrorDir, "fetch", "origin", refspec); err != nil {
		return fmt.Errorf("fetch branch %q: %w", ref, err)
	}
	return nil
}

// ResolveBranchSHA returns the fetched tip SHA for one branch. A --mirror clone
// maps the remote's heads to local refs/heads/*. A missing ref yields a wrapped
// error, which drives per-branch isolation (one bad branch fails only its run).
func ResolveBranchSHA(ctx context.Context, mirrorDir, ref string) (string, error) {
	out, err := runGit(ctx, "", "-C", mirrorDir, "rev-parse", "refs/heads/"+ref)
	if err != nil {
		return "", fmt.Errorf("resolve branch %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CaptureHEAD returns the current HEAD SHA of the git directory at dir.
func CaptureHEAD(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, "", "-C", dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("capture HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateWorktree adds an ephemeral detached worktree at scratchPath, checked out
// at sha, from the git directory at gitDir (a mirror or a working repo).
func CreateWorktree(ctx context.Context, gitDir, sha, scratchPath string) error {
	if err := os.MkdirAll(filepath.Dir(scratchPath), 0o700); err != nil {
		return err
	}
	if _, err := runGit(ctx, "", "-C", gitDir, "worktree", "add", "--detach", scratchPath, sha); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}
	return nil
}

// RemoveWorktree force-removes the worktree at scratchPath.
func RemoveWorktree(ctx context.Context, gitDir, scratchPath string) error {
	if _, err := runGit(ctx, "", "-C", gitDir, "worktree", "remove", "--force", scratchPath); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

// PruneWorktrees prunes worktree administrative entries whose directories are gone.
func PruneWorktrees(ctx context.Context, gitDir string) error {
	if _, err := runGit(ctx, "", "-C", gitDir, "worktree", "prune"); err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}
	return nil
}
