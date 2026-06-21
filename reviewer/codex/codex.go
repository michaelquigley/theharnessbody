// Package codex is a reviewer backed by the `codex exec` CLI. It is lifted from
// mercurius's proven adapter (the codex integration that runs in heavy
// production use), with otis's per-reviewer env merge folded in and config.toml
// made optional.
package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/michaelquigley/theharnessbody/reviewer"
	"github.com/michaelquigley/theharnessbody/reviewer/jsonout"
)

const defaultBinaryPath = "codex"

// Options configures the codex subprocess reviewer.
type Options struct {
	BinaryPath string
	Model      string
	ExtraArgs  []string
	// Env carries additional environment entries ("KEY=VALUE") merged into the
	// codex process environment before CODEX_HOME is set. A reviewer running
	// under a stripped service environment (for example a systemd --user unit
	// that lacks the interactive PATH) uses this to give the codex CLI a usable
	// PATH. Lifted from otis, which hit exactly this.
	Env []string
}

// Reviewer invokes `codex exec` for one structured review.
type Reviewer struct {
	options Options
}

// New returns a codex subprocess reviewer.
func New(options Options) *Reviewer {
	if options.BinaryPath == "" {
		options.BinaryPath = defaultBinaryPath
	}
	options.ExtraArgs = append([]string(nil), options.ExtraArgs...)
	options.Env = append([]string(nil), options.Env...)
	return &Reviewer{options: options}
}

// Review runs `codex exec` with the pre-assembled prompt and supplied schema. The
// returned Raw is unvalidated; the caller validates it against req.Schema.
func (r *Reviewer) Review(ctx context.Context, req reviewer.ReviewRequest) (reviewer.ReviewResponse, error) {
	if err := ctx.Err(); err != nil {
		return reviewer.ReviewResponse{}, err
	}
	if req.WorkingDir == "" {
		return reviewer.ReviewResponse{}, errors.New("codex reviewer working directory is required")
	}
	if len(req.Schema) == 0 {
		return reviewer.ReviewResponse{}, errors.New("codex reviewer schema is required")
	}

	tempDir, err := os.MkdirTemp("", "theharnessbody-codex-*")
	if err != nil {
		return reviewer.ReviewResponse{}, fmt.Errorf("create codex temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	schemaPath := filepath.Join(tempDir, "schema.json")
	lastMessagePath := filepath.Join(tempDir, "last-message.json")
	if err := os.WriteFile(schemaPath, req.Schema, 0o600); err != nil {
		return reviewer.ReviewResponse{}, fmt.Errorf("write codex schema file: %w", err)
	}

	stdout, stderr, runErr := r.run(ctx, req.WorkingDir, req.Prompt, schemaPath, lastMessagePath)
	if runErr != nil {
		// Command failure wins. A nonzero exit, timeout, or cancellation is a
		// failed review, not a successful one — even if a last-message file was
		// written. runErr already carries codex's stdout/stderr as diagnostic.
		return reviewer.ReviewResponse{}, runErr
	}

	output, err := os.ReadFile(lastMessagePath)
	if err != nil {
		return reviewer.ReviewResponse{}, fmt.Errorf("read codex last message file: %w", err)
	}
	raw, err := jsonout.Object(output)
	if err != nil {
		return reviewer.ReviewResponse{}, err
	}

	return reviewer.ReviewResponse{
		Raw:        raw,
		UsageNotes: r.usageNotes(stdout, stderr),
	}, nil
}

func (r *Reviewer) run(ctx context.Context, workingDir string, prompt string, schemaPath string, lastMessagePath string) ([]byte, []byte, error) {
	args := r.args(workingDir, schemaPath, lastMessagePath)
	cmd := exec.CommandContext(ctx, r.options.BinaryPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	codexHome, cleanup, err := r.prepareCodexHome(workingDir)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()
	cmd.Env = append(os.Environ(), r.options.Env...)
	cmd.Env = append(cmd.Env, "CODEX_HOME="+codexHome)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("codex reviewer failed: %w%s", err, commandOutputSuffix(stdout.Bytes(), stderr.Bytes()))
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func (r *Reviewer) prepareCodexHome(workingDir string) (string, func(), error) {
	originalHome, err := codexHome()
	if err != nil {
		return "", func() {}, err
	}

	dir, err := os.MkdirTemp(workingDir, ".codex-home-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create codex home: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	// auth.json is a real dependency — codex needs credentials. config.toml is
	// optional: link it only when present, otherwise codex falls back to its
	// defaults (this reviewer passes model and sandbox explicitly). otis learned
	// the hard way that a hard-required config.toml fails on hosts that run codex
	// on defaults.
	if err := linkCodexHomeEntry(originalHome, dir, "auth.json"); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := linkOptionalCodexHomeEntry(originalHome, dir, "config.toml"); err != nil {
		cleanup()
		return "", func() {}, err
	}
	for _, subdir := range []string{"sessions", "log", ".tmp", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o700); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("create codex home subdir '%s': %w", subdir, err)
		}
	}
	return dir, cleanup, nil
}

func (r *Reviewer) args(workingDir string, schemaPath string, lastMessagePath string) []string {
	args := []string{
		"exec",
		"-C", workingDir,
		"--ephemeral",
		"--skip-git-repo-check",
		"--sandbox", "read-only",
		"--output-schema", schemaPath,
		"--output-last-message", lastMessagePath,
	}
	if r.options.Model != "" {
		args = append(args, "-m", r.options.Model)
	}
	args = append(args, r.options.ExtraArgs...)
	return args
}

func (r *Reviewer) usageNotes(stdout []byte, stderr []byte) string {
	parts := []string{fmt.Sprintf("binary='%s'", r.options.BinaryPath)}
	if r.options.Model != "" {
		parts = append(parts, fmt.Sprintf("model='%s'", r.options.Model))
	}
	parts = append(parts, fmt.Sprintf("stdout_bytes='%d'", len(stdout)))
	parts = append(parts, fmt.Sprintf("stderr_bytes='%d'", len(stderr)))
	return strings.Join(parts, ", ")
}

func codexHome() (string, error) {
	if path := os.Getenv("CODEX_HOME"); path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve codex home: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func linkCodexHomeEntry(sourceHome string, targetHome string, name string) error {
	source := filepath.Join(sourceHome, name)
	target := filepath.Join(targetHome, name)
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("codex home entry '%s' is not available: %w", source, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("link codex home entry '%s': %w", name, err)
	}
	return nil
}

func linkOptionalCodexHomeEntry(sourceHome string, targetHome string, name string) error {
	source := filepath.Join(sourceHome, name)
	if _, err := os.Stat(source); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect codex home entry '%s': %w", source, err)
	}
	return linkCodexHomeEntry(sourceHome, targetHome, name)
}

func commandOutputSuffix(stdout []byte, stderr []byte) string {
	var parts []string
	if text := strings.TrimSpace(string(stderr)); text != "" {
		parts = append(parts, fmt.Sprintf("stderr: %s", text))
	}
	if text := strings.TrimSpace(string(stdout)); text != "" {
		parts = append(parts, fmt.Sprintf("stdout: %s", text))
	}
	if len(parts) == 0 {
		return ""
	}
	return "; " + strings.Join(parts, "; ")
}
