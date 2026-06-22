package repo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// buildSSHCommand builds a GIT_SSH_COMMAND value that authenticates git with a
// specific private key, offers only that key (so git never falls back to a
// running ssh-agent), and accepts an unknown host key on first connect (so an
// automated run doesn't hang on a host-key prompt). The key path is shell-quoted
// because git parses GIT_SSH_COMMAND with sh-style word splitting. An empty key
// yields an empty command, leaving git's environment untouched.
func buildSSHCommand(keyPath string) string {
	if strings.TrimSpace(keyPath) == "" {
		return ""
	}
	return fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", shellQuote(keyPath))
}

// shellQuote wraps a value in single quotes for safe sh-style parsing, escaping
// any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sshEnv returns the GIT_SSH_COMMAND entry to append to os.Environ(), or nil for
// an empty key. The result must be appended to os.Environ(), never used as a bare
// cmd.Env — that would drop HOME/PATH and break git and ssh.
func sshEnv(sshKey string) []string {
	cmd := buildSSHCommand(sshKey)
	if cmd == "" {
		return nil
	}
	return []string{"GIT_SSH_COMMAND=" + cmd}
}

// runGit runs a git command with an optional ssh key and returns stdout. It is
// used by the path-based mirror/worktree functions; the Repo handle uses its own
// runCtx (which binds a working directory and inspects output on error).
func runGit(ctx context.Context, sshKey string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), sshEnv(sshKey)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w%s", strings.Join(args, " "), err, stderrSuffix(stderr.Bytes()))
	}
	return out, nil
}

func stderrSuffix(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	return ": " + string(raw)
}
