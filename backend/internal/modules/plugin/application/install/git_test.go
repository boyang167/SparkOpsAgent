package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMemoryPublisherStoresLastRequest(t *testing.T) {
	publisher := &MemoryPublisher{}
	req := PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files: map[string][]byte{
			"backend/internal/plugins/example/plugin.go": []byte("package example\n"),
		},
		Delete: []string{"backend/internal/plugins/old/plugin.go"},
	}

	url, err := publisher.Publish(context.Background(), req)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if url != "https://example.local/pr/1" {
		t.Fatalf("Publish() url = %q", url)
	}
	if !reflect.DeepEqual(publisher.Last, req) {
		t.Fatalf("Last request = %#v, want %#v", publisher.Last, req)
	}

	req.Files["backend/internal/plugins/example/plugin.go"][0] = 'X'
	if string(publisher.Last.Files["backend/internal/plugins/example/plugin.go"]) != "package example\n" {
		t.Fatalf("MemoryPublisher retained mutable request bytes")
	}
}

func TestGitCLIPublisherRejectsUnsafePaths(t *testing.T) {
	tests := []struct {
		name string
		req  PublishRequest
	}{
		{
			name: "absolute file path",
			req: PublishRequest{
				Branch:  "plugins/example",
				Message: "msg",
				Files:   map[string][]byte{"/backend/internal/plugins/example/plugin.go": []byte("x")},
			},
		},
		{
			name: "windows drive file path",
			req: PublishRequest{
				Branch:  "plugins/example",
				Message: "msg",
				Files:   map[string][]byte{"C:/repo/plugin.go": []byte("x")},
			},
		},
		{
			name: "parent traversal delete path",
			req: PublishRequest{
				Branch:  "plugins/example",
				Message: "msg",
				Delete:  []string{"backend/internal/plugins/../secret.go"},
			},
		},
		{
			name: "backslash file path",
			req: PublishRequest{
				Branch:  "plugins/example",
				Message: "msg",
				Files:   map[string][]byte{`backend\internal\plugins\example\plugin.go`: []byte("x")},
			},
		},
		{
			name: "same path written and deleted",
			req: PublishRequest{
				Branch:  "plugins/example",
				Message: "msg",
				Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("x")},
				Delete:  []string{"backend/internal/plugins/example/plugin.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordingRunner{}
			publisher := &GitCLIPublisher{
				root:    t.TempDir(),
				runner:  runner,
				tempDir: func(string, string) (string, error) { return t.TempDir(), nil },
			}

			if _, err := publisher.Publish(context.Background(), tt.req); err == nil {
				t.Fatalf("Publish() error = nil")
			}
			if len(runner.calls) != 0 {
				t.Fatalf("Publish() ran commands for invalid request: %#v", runner.calls)
			}
		})
	}
}

func TestGitCLIPublisherRejectsInvalidBranchBeforeCommands(t *testing.T) {
	tests := []string{"", " plugins/example", "plugins/example ", "plugins/\nexample", "plugins/\x1fexample", "@{-1}", "plugins/@{bad}"}
	for _, branch := range tests {
		t.Run(branch, func(t *testing.T) {
			runner := &recordingRunner{}
			publisher := &GitCLIPublisher{
				root:    t.TempDir(),
				runner:  runner,
				tempDir: func(string, string) (string, error) { return t.TempDir(), nil },
			}

			if _, err := publisher.Publish(context.Background(), PublishRequest{
				Branch:  branch,
				Message: "msg",
				Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("x")},
			}); err == nil {
				t.Fatalf("Publish() error = nil")
			}
			if len(runner.calls) != 0 {
				t.Fatalf("Publish() ran commands for invalid branch: %#v", runner.calls)
			}
		})
	}
}

func TestGitCLIPublisherUsesIsolatedWorktreeAndCreatesPullRequest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{err: errors.New("missing ref")},
		{}, {}, {}, {}, {},
		{stdout: "https://example.local/pr/42\n"},
		{}, {},
	}}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return worktree, nil },
	}

	url, err := publisher.Publish(ctx, PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files: map[string][]byte{
			"backend/internal/plugins/example/plugin.go": []byte("package example\n"),
			"backend/internal/plugins/register.go":       []byte("package plugins\n"),
		},
		Delete: []string{"backend/internal/plugins/old/plugin.go"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if url != "https://example.local/pr/42" {
		t.Fatalf("Publish() url = %q", url)
	}

	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"show-ref", "--verify", "--quiet", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
		{dir: root, name: "git", args: []string{"worktree", "add", worktree, "-b", "plugins/example"}},
		{dir: worktree, name: "git", args: []string{"add", "--all"}},
		{dir: worktree, name: "git", args: []string{"commit", "-m", "Install example plugin"}},
		{dir: worktree, name: "git", args: []string{"push", "-u", "origin", "plugins/example"}},
		{dir: worktree, name: "gh", args: []string{"pr", "create", "--fill"}},
		{dir: root, name: "git", args: []string{"worktree", "remove", "--force", worktree}},
		{dir: root, name: "git", args: []string{"branch", "-D", "--", "plugins/example"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}

	written, err := os.ReadFile(filepath.Join(worktree, "backend", "internal", "plugins", "example", "plugin.go"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(written) != "package example\n" {
		t.Fatalf("written file = %q", written)
	}
	if _, err := os.Stat(filepath.Join(worktree, "backend", "internal", "plugins", "old", "plugin.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted file stat error = %v, want not exist", err)
	}
}

func TestGitCLIPublisherPassesMissingWorktreePathToGit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runner := &worktreePathRunner{t: t, results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{err: errors.New("missing ref")},
		{}, {}, {}, {}, {},
		{stdout: "https://example.local/pr/42\n"},
		{}, {},
	}}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: os.MkdirTemp,
	}

	if _, err := publisher.Publish(ctx, PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestGitCLIPublisherRejectsDirtyRootAtPublishStart(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{err: errors.New("missing ref")},
		{stdout: " M backend/internal/plugins/example/plugin.go\n"},
	}}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return t.TempDir(), nil },
	}

	if _, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	}); err == nil {
		t.Fatalf("Publish() error = nil")
	}
	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"show-ref", "--verify", "--quiet", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestGitCLIPublisherRejectsExistingBranchWithoutDeletingIt(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{},
	}}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return t.TempDir(), nil },
	}

	if _, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	}); err == nil {
		t.Fatalf("Publish() error = nil")
	}
	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"show-ref", "--verify", "--quiet", "refs/heads/plugins/example"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestGitCLIPublisherRejectsUnexpectedNormalizedBranchRef(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/other\n"},
	}}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return t.TempDir(), nil },
	}

	if _, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	}); err == nil {
		t.Fatalf("Publish() error = nil")
	}
	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugins/example"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestGitCLIPublisherRejectsSymlinkDuringWrite(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	runner := &recordingRunner{
		results: []commandResult{{stdout: "refs/heads/plugins/example\n"}, {err: errors.New("missing ref")}, {}, {}, {}, {}},
		afterRun: func(call commandCall) {
			if call.name != "git" || !reflect.DeepEqual(call.args[:2], []string{"worktree", "add"}) {
				return
			}
			target := filepath.Join(t.TempDir(), "outside")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatalf("mkdir target: %v", err)
			}
			if err := os.MkdirAll(filepath.Join(worktree, "backend"), 0o755); err != nil {
				t.Fatalf("mkdir worktree backend: %v", err)
			}
			link := filepath.Join(worktree, "backend", "internal")
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
		},
	}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return worktree, nil },
	}

	if _, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	}); err == nil {
		t.Fatalf("Publish() error = nil")
	}
}

func TestGitCLIPublisherDeletesNonEmptyDirectorySafely(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	runner := &recordingRunner{
		results: []commandResult{
			{stdout: "refs/heads/plugins/example\n"},
			{err: errors.New("missing ref")},
			{}, {}, {}, {}, {},
			{stdout: "https://example.local/pr/42\n"},
			{}, {},
		},
		afterRun: func(call commandCall) {
			if call.name != "git" || !reflect.DeepEqual(call.args[:2], []string{"worktree", "add"}) {
				return
			}
			dir := filepath.Join(worktree, "backend", "internal", "plugins", "old")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir delete dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "nested.go"), []byte("package old\n"), 0o644); err != nil {
				t.Fatalf("write nested file: %v", err)
			}
		},
	}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return worktree, nil },
	}

	if _, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Remove old plugin",
		Delete:  []string{"backend/internal/plugins/old"},
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "backend", "internal", "plugins", "old")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted dir stat error = %v, want not exist", err)
	}
}

func TestGitCLIPublisherRejectsSymlinkInsideDeletedDirectory(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	runner := &recordingRunner{
		results: []commandResult{{stdout: "refs/heads/plugins/example\n"}, {err: errors.New("missing ref")}, {}, {}, {}, {}},
		afterRun: func(call commandCall) {
			if call.name != "git" || !reflect.DeepEqual(call.args[:2], []string{"worktree", "add"}) {
				return
			}
			dir := filepath.Join(worktree, "backend", "internal", "plugins", "old")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir delete dir: %v", err)
			}
			target := filepath.Join(t.TempDir(), "outside.go")
			if err := os.WriteFile(target, []byte("package outside\n"), 0o644); err != nil {
				t.Fatalf("write symlink target: %v", err)
			}
			if err := os.Symlink(target, filepath.Join(dir, "link.go")); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
		},
	}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return worktree, nil },
	}

	if _, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Remove old plugin",
		Delete:  []string{"backend/internal/plugins/old"},
	}); err == nil {
		t.Fatalf("Publish() error = nil")
	}
}

func TestGitCLIPublisherRejectsInvalidPullRequestURL(t *testing.T) {
	tests := []struct {
		name   string
		result commandResult
	}{
		{name: "empty stdout", result: commandResult{}},
		{name: "garbage stdout", result: commandResult{stdout: "not a url\n"}},
		{name: "warning and url", result: commandResult{stdout: "warning: something\nhttps://example.local/pr/42\n"}},
		{name: "missing host", result: commandResult{stdout: "https:///pr/42\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			worktree := filepath.Join(t.TempDir(), "plugin-worktree")
			runner := &recordingRunner{results: []commandResult{{stdout: "refs/heads/plugins/example\n"}, {err: errors.New("missing ref")}, {}, {}, {}, {}, {}, tt.result, {}, {}}}
			publisher := &GitCLIPublisher{
				root:    root,
				runner:  runner,
				tempDir: func(string, string) (string, error) { return worktree, nil },
			}

			if _, err := publisher.Publish(context.Background(), PublishRequest{
				Branch:  "plugins/example",
				Message: "Install example plugin",
				Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
			}); err == nil {
				t.Fatalf("Publish() error = nil")
			}
			if got := runner.calls[len(runner.calls)-2:]; !reflect.DeepEqual(got, []commandCall{
				{dir: root, name: "git", args: []string{"worktree", "remove", "--force", worktree}},
				{dir: root, name: "git", args: []string{"branch", "-D", "--", "plugins/example"}},
			}) {
				t.Fatalf("cleanup calls = %#v", got)
			}
		})
	}
}

func TestGitCLIPublisherAcceptsPullRequestURLWithStderrWarning(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{err: errors.New("missing ref")},
		{}, {}, {}, {}, {},
		{stdout: "https://example.local/pr/42\n", stderr: "warning: something\n"},
		{}, {},
	}}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return worktree, nil },
	}

	url, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if url != "https://example.local/pr/42" {
		t.Fatalf("Publish() url = %q", url)
	}
}

func TestGitCLIPublisherCleansUpWithBackgroundContextAndJoinsErrors(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	ctx, cancel := context.WithCancel(context.Background())
	runner := &recordingRunner{
		results: []commandResult{
			{stdout: "refs/heads/plugins/example\n"},
			{err: errors.New("missing ref")},
			{}, {}, {},
			{err: errors.New("commit failed")},
			{err: errors.New("remove failed")},
			{err: errors.New("branch delete failed")},
		},
		afterRun: func(call commandCall) {
			if call.name == "git" && len(call.args) > 0 && call.args[0] == "commit" {
				cancel()
			}
		},
	}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return worktree, nil },
	}

	_, err := publisher.Publish(ctx, PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	})
	if err == nil {
		t.Fatalf("Publish() error = nil")
	}
	for _, part := range []string{"commit failed", "remove failed", "branch delete failed"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("Publish() error %q does not contain %q", err, part)
		}
	}
	if runner.cleanupCtxErr != nil {
		t.Fatalf("cleanup used canceled request context: %v", runner.cleanupCtxErr)
	}
}

func TestGitCLIPublisherCleansUpAfterCommandFailures(t *testing.T) {
	tests := []struct {
		name    string
		results []commandResult
	}{
		{
			name: "git add failure",
			results: []commandResult{
				{stdout: "refs/heads/plugins/example\n"},
				{err: errors.New("missing ref")},
				{}, {},
				{err: errors.New("add failed")},
				{}, {},
			},
		},
		{
			name: "git push failure",
			results: []commandResult{
				{stdout: "refs/heads/plugins/example\n"},
				{err: errors.New("missing ref")},
				{}, {}, {}, {},
				{err: errors.New("push failed")},
				{}, {},
			},
		},
		{
			name: "gh failure",
			results: []commandResult{
				{stdout: "refs/heads/plugins/example\n"},
				{err: errors.New("missing ref")},
				{}, {}, {}, {}, {},
				{stderr: "failed to create pr\n", err: errors.New("gh failed")},
				{}, {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			worktree := filepath.Join(t.TempDir(), "plugin-worktree")
			runner := &recordingRunner{results: tt.results}
			publisher := &GitCLIPublisher{
				root:    root,
				runner:  runner,
				tempDir: func(string, string) (string, error) { return worktree, nil },
			}

			if _, err := publisher.Publish(context.Background(), PublishRequest{
				Branch:  "plugins/example",
				Message: "Install example plugin",
				Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
			}); err == nil {
				t.Fatalf("Publish() error = nil")
			}
			if got := runner.calls[len(runner.calls)-2:]; !reflect.DeepEqual(got, []commandCall{
				{dir: root, name: "git", args: []string{"worktree", "remove", "--force", worktree}},
				{dir: root, name: "git", args: []string{"branch", "-D", "--", "plugins/example"}},
			}) {
				t.Fatalf("cleanup calls = %#v", got)
			}
		})
	}
}

func TestGitCLIPublisherCleansUpAfterWorktreeAddFailure(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{err: errors.New("missing ref")},
		{},
		{err: errors.New("worktree add failed")},
		{}, {},
		{},
	}}
	publisher := &GitCLIPublisher{
		root:    root,
		runner:  runner,
		tempDir: func(string, string) (string, error) { return worktree, nil },
	}

	if _, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	}); err == nil {
		t.Fatalf("Publish() error = nil")
	}
	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"show-ref", "--verify", "--quiet", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
		{dir: root, name: "git", args: []string{"worktree", "add", worktree, "-b", "plugins/example"}},
		{dir: root, name: "git", args: []string{"worktree", "remove", "--force", worktree}},
		{dir: root, name: "git", args: []string{"show-ref", "--verify", "--quiet", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"branch", "-D", "--", "plugins/example"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestNewGitCLIPublisherFromEnvRequiresCleanGitRoot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runner := &recordingRunner{results: []commandResult{{stdout: "true\n"}, {}}}
	t.Setenv("PLUGIN_GIT_ROOT", root)

	publisher, err := NewGitCLIPublisherFromEnv(ctx, runner)
	if err != nil {
		t.Fatalf("NewGitCLIPublisherFromEnv() error = %v", err)
	}
	if publisher.root != root {
		t.Fatalf("root = %q, want %q", publisher.root, root)
	}
	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"rev-parse", "--is-inside-work-tree"}},
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestNewGitCLIPublisherFromEnvRejectsMissingOrDirtyRoot(t *testing.T) {
	ctx := context.Background()
	if _, err := NewGitCLIPublisherFromEnv(ctx, &recordingRunner{}); err == nil {
		t.Fatalf("NewGitCLIPublisherFromEnv() missing env error = nil")
	}

	t.Setenv("PLUGIN_GIT_ROOT", t.TempDir())
	runner := &recordingRunner{results: []commandResult{{stdout: "true\n"}, {stdout: " M dirty.go\n"}}}
	if _, err := NewGitCLIPublisherFromEnv(ctx, runner); err == nil {
		t.Fatalf("NewGitCLIPublisherFromEnv() dirty root error = nil")
	}
}

type commandCall struct {
	dir  string
	name string
	args []string
}

type commandResult struct {
	stdout string
	stderr string
	err    error
}

type recordingRunner struct {
	calls         []commandCall
	results       []commandResult
	afterRun      func(commandCall)
	cleanupCtxErr error
}

func (r *recordingRunner) Run(ctx context.Context, dir string, name string, args ...string) (CommandOutput, error) {
	copiedArgs := append([]string(nil), args...)
	call := commandCall{dir: dir, name: name, args: copiedArgs}
	r.calls = append(r.calls, call)
	if name == "git" && len(args) > 0 && (args[0] == "worktree" || args[0] == "branch") && len(r.calls) > 5 && ctx.Err() != nil {
		r.cleanupCtxErr = ctx.Err()
	}
	if r.afterRun != nil {
		r.afterRun(call)
	}
	if len(r.results) == 0 {
		return CommandOutput{}, nil
	}
	result := r.results[0]
	r.results = r.results[1:]
	return CommandOutput{Stdout: []byte(result.stdout), Stderr: []byte(result.stderr)}, result.err
}

type worktreePathRunner struct {
	t       *testing.T
	results []commandResult
}

func (r *worktreePathRunner) Run(ctx context.Context, dir string, name string, args ...string) (CommandOutput, error) {
	_ = ctx
	_ = dir
	if name == "git" && len(args) >= 2 && reflect.DeepEqual(args[:2], []string{"worktree", "add"}) {
		if _, err := os.Stat(args[2]); !errors.Is(err, os.ErrNotExist) {
			r.t.Fatalf("worktree path exists before git worktree add: %v", err)
		}
	}
	if len(r.results) == 0 {
		return CommandOutput{}, nil
	}
	result := r.results[0]
	r.results = r.results[1:]
	return CommandOutput{Stdout: []byte(result.stdout), Stderr: []byte(result.stderr)}, result.err
}
