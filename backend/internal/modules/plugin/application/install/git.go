package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type GitPublisher interface {
	Publish(ctx context.Context, req PublishRequest) (prURL string, err error)
}

type PublishRequest struct {
	Branch  string
	Message string
	Files   map[string][]byte
	Delete  []string
}

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

type GitCLIPublisher struct {
	root    string
	runner  CommandRunner
	tempDir func(dir, pattern string) (string, error)
}

func NewGitCLIPublisherFromEnv(ctx context.Context, runner CommandRunner) (*GitCLIPublisher, error) {
	root := strings.TrimSpace(os.Getenv("PLUGIN_GIT_ROOT"))
	if root == "" {
		return nil, fmt.Errorf("PLUGIN_GIT_ROOT is required")
	}
	if runner == nil {
		runner = execCommandRunner{}
	}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: os.MkdirTemp,
	}
	if err := publisher.validateRoot(ctx); err != nil {
		return nil, err
	}
	return publisher, nil
}

func (p *GitCLIPublisher) Publish(ctx context.Context, req PublishRequest) (prURL string, err error) {
	if err := validatePublishRequest(req); err != nil {
		return "", err
	}
	runner := p.commandRunner()
	tempDir := p.tempDir
	if tempDir == nil {
		tempDir = os.MkdirTemp
	}

	worktree, err := tempDir("", "sparkops-plugin-*")
	if err != nil {
		return "", err
	}
	if err := os.Remove(worktree); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	cleanupWorktree := false
	defer func() {
		if !cleanupWorktree {
			return
		}
		cleanupErr := runCommand(ctx, runner, p.root, "git", "worktree", "remove", "--force", worktree)
		if err == nil && cleanupErr != nil {
			err = cleanupErr
		}
	}()

	if err := runCommand(ctx, runner, p.root, "git", "worktree", "add", worktree, "-b", req.Branch); err != nil {
		return "", err
	}
	cleanupWorktree = true

	for name, content := range req.Files {
		target := filepath.Join(worktree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return "", err
		}
	}
	for _, name := range req.Delete {
		target := filepath.Join(worktree, filepath.FromSlash(name))
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}

	if err := runCommand(ctx, runner, worktree, "git", "add", "--all"); err != nil {
		return "", err
	}
	if err := runCommand(ctx, runner, worktree, "git", "commit", "-m", req.Message); err != nil {
		return "", err
	}
	if err := runCommand(ctx, runner, worktree, "git", "push", "-u", "origin", req.Branch); err != nil {
		return "", err
	}
	out, err := runner.Run(ctx, worktree, "gh", "pr", "create", "--fill")
	if err != nil {
		return "", fmt.Errorf("gh pr create --fill: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *GitCLIPublisher) validateRoot(ctx context.Context) error {
	runner := p.commandRunner()
	out, err := runner.Run(ctx, p.root, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return fmt.Errorf("validate PLUGIN_GIT_ROOT: %w", err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("PLUGIN_GIT_ROOT is not a git work tree")
	}
	out, err = runner.Run(ctx, p.root, "git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("validate PLUGIN_GIT_ROOT cleanliness: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("PLUGIN_GIT_ROOT must be clean")
	}
	return nil
}

func (p *GitCLIPublisher) commandRunner() CommandRunner {
	if p.runner != nil {
		return p.runner
	}
	return execCommandRunner{}
}

func validatePublishRequest(req PublishRequest) error {
	if strings.TrimSpace(req.Branch) == "" {
		return fmt.Errorf("publish branch is required")
	}
	if strings.TrimSpace(req.Message) == "" {
		return fmt.Errorf("publish message is required")
	}
	seenWrites := make(map[string]struct{}, len(req.Files))
	for name := range req.Files {
		if !isSafeRelativePath(name) {
			return fmt.Errorf("unsafe publish file path: %q", name)
		}
		seenWrites[name] = struct{}{}
	}
	for _, name := range req.Delete {
		if !isSafeRelativePath(name) {
			return fmt.Errorf("unsafe publish delete path: %q", name)
		}
		if _, ok := seenWrites[name]; ok {
			return fmt.Errorf("publish path cannot be both written and deleted: %q", name)
		}
	}
	return nil
}

func isSafeRelativePath(name string) bool {
	if name == "" || strings.Contains(name, `\`) || strings.HasPrefix(name, "/") || hasWindowsDrive(name) {
		return false
	}
	if path.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func runCommand(ctx context.Context, runner CommandRunner, dir string, name string, args ...string) error {
	if _, err := runner.Run(ctx, dir, name, args...); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
