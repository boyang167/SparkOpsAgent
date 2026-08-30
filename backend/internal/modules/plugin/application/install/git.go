package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
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

type CommandOutput struct {
	Stdout []byte
	Stderr []byte
}

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (CommandOutput, error)
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
	if err := validateBranchWithGit(ctx, runner, p.root, req.Branch); err != nil {
		return "", err
	}
	if err := ensureCleanGitRoot(ctx, runner, p.root); err != nil {
		return "", err
	}
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

	if _, err := runCommand(ctx, runner, p.root, "git", "branch", "--", req.Branch, "HEAD"); err != nil {
		return "", err
	}
	branchOwned := true
	defer func() {
		cleanupErr := cleanupGitWorktreeAndBranch(runner, p.root, worktree, req.Branch, branchOwned)
		if cleanupErr == nil {
			return
		}
		if err != nil {
			err = errors.Join(err, cleanupErr)
			return
		}
		prURL = ""
		err = cleanupErr
	}()

	if _, err := runCommand(ctx, runner, p.root, "git", "worktree", "add", worktree, req.Branch); err != nil {
		return "", err
	}

	for name, content := range req.Files {
		if err := writeWorktreeFile(worktree, name, content); err != nil {
			return "", err
		}
	}
	for _, name := range req.Delete {
		if err := deleteWorktreePath(worktree, name); err != nil {
			return "", err
		}
	}

	if _, err := runCommand(ctx, runner, worktree, "git", "add", "--all"); err != nil {
		return "", err
	}
	if _, err := runCommand(ctx, runner, worktree, "git", "commit", "-m", req.Message); err != nil {
		return "", err
	}
	if _, err := runCommand(ctx, runner, worktree, "git", "push", "-u", "origin", req.Branch); err != nil {
		return "", err
	}
	out, err := runCommand(ctx, runner, worktree, "gh", "pr", "create", "--fill")
	if err != nil {
		return "", err
	}
	return parsePRURL(out)
}

func cleanupGitWorktreeAndBranch(runner CommandRunner, root string, worktree string, branch string, branchOwned bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cleanupErr error
	if _, err := runCommand(ctx, runner, root, "git", "worktree", "remove", "--force", worktree); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if branchOwned {
		if _, err := runCommand(ctx, runner, root, "git", "branch", "-D", "--", branch); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func writeWorktreeFile(worktree string, name string, content []byte) error {
	target, err := safeWorktreePath(worktree, name)
	if err != nil {
		return err
	}
	if err := ensureSafeParents(worktree, target); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write symlink path: %s", name)
		}
		if info.IsDir() {
			return fmt.Errorf("refusing to overwrite directory: %s", name)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, content, 0o644)
}

func deleteWorktreePath(worktree string, name string) error {
	target, err := safeWorktreePath(worktree, name)
	if err != nil {
		return err
	}
	if err := ensureSafeParents(worktree, target); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to delete symlink path: %s", name)
	}
	if !info.IsDir() {
		return os.Remove(target)
	}
	if err := rejectSymlinkTree(target); err != nil {
		return err
	}
	return os.RemoveAll(target)
}

func safeWorktreePath(worktree string, name string) (string, error) {
	if !isSafeRelativePath(name) {
		return "", fmt.Errorf("unsafe publish path: %q", name)
	}
	root, err := filepath.Abs(worktree)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("publish path escapes worktree root: %q", name)
	}
	return target, nil
}

func ensureSafeParents(worktree string, target string) error {
	root, err := filepath.Abs(worktree)
	if err != nil {
		return err
	}
	parent := filepath.Dir(target)
	rel, err := filepath.Rel(root, parent)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}

	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink traversal: %s", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("path parent is not a directory: %s", current)
		}
	}
	return nil
}

func rejectSymlinkTree(root string) error {
	return filepath.WalkDir(root, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in deleted directory: %s", name)
		}
		return nil
	})
}

func (p *GitCLIPublisher) validateRoot(ctx context.Context) error {
	runner := p.commandRunner()
	out, err := runner.Run(ctx, p.root, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return fmt.Errorf("validate PLUGIN_GIT_ROOT: %w", err)
	}
	if strings.TrimSpace(string(out.Stdout)) != "true" {
		return fmt.Errorf("PLUGIN_GIT_ROOT is not a git work tree")
	}
	return ensureCleanGitRoot(ctx, runner, p.root)
}

func (p *GitCLIPublisher) commandRunner() CommandRunner {
	if p.runner != nil {
		return p.runner
	}
	return execCommandRunner{}
}

func validatePublishRequest(req PublishRequest) error {
	if err := validateBranchName(req.Branch); err != nil {
		return err
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

func validateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("publish branch is required")
	}
	if strings.Contains(branch, "@{") {
		return fmt.Errorf("publish branch contains unsupported ref expression")
	}
	if strings.TrimSpace(branch) != branch {
		return fmt.Errorf("publish branch must not contain surrounding whitespace")
	}
	for _, r := range branch {
		if unicode.IsSpace(r) || r < 0x20 || r == 0x7f {
			return fmt.Errorf("publish branch contains whitespace or control character")
		}
	}
	return nil
}

func validateBranchWithGit(ctx context.Context, runner CommandRunner, root string, branch string) error {
	ref := branchRef(branch)
	out, err := runCommand(ctx, runner, root, "git", "check-ref-format", "--normalize", ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out.Stdout)) != ref {
		return fmt.Errorf("git normalized branch ref %q does not match %q", strings.TrimSpace(string(out.Stdout)), ref)
	}
	return nil
}

func branchRef(branch string) string {
	return "refs/heads/" + branch
}

func ensureCleanGitRoot(ctx context.Context, runner CommandRunner, root string) error {
	out, err := runner.Run(ctx, root, "git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("validate git root cleanliness: %w", err)
	}
	if strings.TrimSpace(string(out.Stdout)) != "" {
		return fmt.Errorf("git root must be clean")
	}
	return nil
}

func isSafeRelativePath(name string) bool {
	if name == "" || strings.Contains(name, `\`) || strings.HasPrefix(name, "/") || hasWindowsDrivePath(name) {
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

func hasWindowsDrivePath(name string) bool {
	return len(name) >= 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z'))
}

func parsePRURL(out CommandOutput) (string, error) {
	raw := strings.TrimSpace(string(out.Stdout))
	if raw == "" {
		return "", fmt.Errorf("gh pr create returned empty URL")
	}
	if len(strings.Fields(raw)) != 1 {
		return "", fmt.Errorf("gh pr create returned invalid URL output: %q", raw)
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("gh pr create returned invalid URL: %q", raw)
	}
	return raw, nil
}

func runCommand(ctx context.Context, runner CommandRunner, dir string, name string, args ...string) (CommandOutput, error) {
	out, err := runner.Run(ctx, dir, name, args...)
	if err != nil {
		return out, fmt.Errorf("%s %s: %w%s", name, strings.Join(args, " "), err, commandOutputSuffix(out))
	}
	return out, nil
}

func commandOutputSuffix(out CommandOutput) string {
	stderr := strings.TrimSpace(string(out.Stderr))
	if stderr != "" {
		return ": " + stderr
	}
	stdout := strings.TrimSpace(string(out.Stdout))
	if stdout != "" {
		return ": " + stdout
	}
	return ""
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, dir string, name string, args ...string) (CommandOutput, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := CommandOutput{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err != nil {
		return out, err
	}
	return out, nil
}
