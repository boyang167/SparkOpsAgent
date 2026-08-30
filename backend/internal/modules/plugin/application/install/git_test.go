package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
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

func TestMemoryPublisherLooksUpPublishedPullRequestByBranch(t *testing.T) {
	publisher := &MemoryPublisher{}
	_, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugin/install-example",
		Message: "Install example",
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	view, err := publisher.LookupPullRequest(context.Background(), "plugin/install-example")

	if err != nil {
		t.Fatalf("LookupPullRequest() error = %v", err)
	}
	want := PullRequestView{
		URL:         "https://example.local/pr/1",
		State:       "OPEN",
		HeadRefName: "plugin/install-example",
	}
	if view != want {
		t.Fatalf("LookupPullRequest() = %#v, want %#v", view, want)
	}
}

func TestGitCLIPublisherLooksUpPullRequestWithStrictURLParsing(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugin/install-example\n"},
		{stdout: `{"url":"https://example.test/pr/42","state":"OPEN","headRefName":"plugin/install-example"}`},
	}}
	publisher := &GitCLIPublisher{root: root, runner: runner}

	view, err := publisher.LookupPullRequest(context.Background(), "plugin/install-example")

	if err != nil {
		t.Fatalf("LookupPullRequest() error = %v", err)
	}
	if view.URL != "https://example.test/pr/42" || view.State != "OPEN" || view.HeadRefName != "plugin/install-example" {
		t.Fatalf("LookupPullRequest() = %#v", view)
	}
	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugin/install-example"}},
		{dir: root, name: "gh", args: []string{"pr", "view", "plugin/install-example", "--json", "url,state,headRefName"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestGitCLIPublisherLookupReportsMissingPullRequest(t *testing.T) {
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugin/install-example\n"},
		{stderr: "no pull requests found for branch \"plugin/install-example\"", err: errors.New("exit 1")},
	}}
	publisher := &GitCLIPublisher{root: t.TempDir(), runner: runner}

	_, err := publisher.LookupPullRequest(context.Background(), "plugin/install-example")

	if !errors.Is(err, ErrPullRequestNotFound) {
		t.Fatalf("LookupPullRequest() error = %v, want ErrPullRequestNotFound", err)
	}
}

func TestGitCLIPublisherLookupRejectsMalformedJSON(t *testing.T) {
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugin/install-example\n"},
		{stdout: "not-a-url\n"},
	}}
	publisher := &GitCLIPublisher{root: t.TempDir(), runner: runner}

	_, err := publisher.LookupPullRequest(context.Background(), "plugin/install-example")

	if err == nil || errors.Is(err, ErrPullRequestNotFound) {
		t.Fatalf("LookupPullRequest() error = %v", err)
	}
}

func TestGitCLIPublisherLookupRejectsNonOpenOrWrongHeadPullRequest(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{name: "closed", json: `{"url":"https://example.test/pr/1","state":"CLOSED","headRefName":"plugin/install-example"}`},
		{name: "merged", json: `{"url":"https://example.test/pr/1","state":"MERGED","headRefName":"plugin/install-example"}`},
		{name: "wrong head", json: `{"url":"https://example.test/pr/1","state":"OPEN","headRefName":"plugin/install-other"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordingRunner{results: []commandResult{
				{stdout: "refs/heads/plugin/install-example\n"},
				{stdout: tt.json},
			}}
			publisher := &GitCLIPublisher{root: t.TempDir(), runner: runner}

			_, err := publisher.LookupPullRequest(context.Background(), "plugin/install-example")

			if !errors.Is(err, ErrPullRequestNotRecoverable) {
				t.Fatalf("LookupPullRequest() error = %v, want ErrPullRequestNotRecoverable", err)
			}
		})
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
		{}, {}, {}, {}, {}, {}, {},
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
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
		{dir: root, name: "git", args: []string{"ls-remote", "--heads", "origin", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"branch", "--", "plugins/example", "HEAD"}},
		{dir: root, name: "git", args: []string{"worktree", "add", worktree, "plugins/example"}},
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
		{}, {}, {}, {}, {}, {}, {},
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
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestGitCLIPublisherDoesNotCleanupBranchWhenBranchCreateFails(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{},
		{},
		{stderr: "fatal: a branch named 'plugins/example' already exists\n", err: errors.New("branch exists")},
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
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
		{dir: root, name: "git", args: []string{"ls-remote", "--heads", "origin", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"branch", "--", "plugins/example", "HEAD"}},
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
		results: []commandResult{{stdout: "refs/heads/plugins/example\n"}, {}, {}, {}, {}, {}, {}},
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
			{}, {}, {}, {}, {}, {}, {},
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
		results: []commandResult{{stdout: "refs/heads/plugins/example\n"}, {}, {}, {}, {}, {}, {}},
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
			runner := &recordingRunner{results: []commandResult{{stdout: "refs/heads/plugins/example\n"}, {}, {}, {}, {}, {}, {}, {}, tt.result, {stdout: "refs/heads/plugins/example\n"}, {stderr: "no pull requests found", err: errors.New("exit 1")}, {}, {}}}
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
		{}, {}, {}, {}, {}, {}, {},
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

func TestGitCLIPublisherCreatesPRFromExistingRemoteBranch(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{results: []commandResult{
		{stdout: "refs/heads/plugins/example\n"},
		{},
		{stdout: "abc123\trefs/heads/plugins/example\n"},
		{stdout: "refs/heads/plugins/example\n"},
		{stderr: "no pull requests found", err: errors.New("exit 1")},
		{stdout: "https://example.local/pr/9\n"},
	}}
	publisher := &GitCLIPublisher{root: root, runner: runner}

	url, err := publisher.Publish(context.Background(), PublishRequest{
		Branch:  "plugins/example",
		Message: "Install example plugin",
		Files:   map[string][]byte{"backend/internal/plugins/example/plugin.go": []byte("package example\n")},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if url != "https://example.local/pr/9" {
		t.Fatalf("Publish() url = %q", url)
	}
	wantCalls := []commandCall{
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
		{dir: root, name: "git", args: []string{"ls-remote", "--heads", "origin", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"check-ref-format", "--normalize", "refs/heads/plugins/example"}},
		{dir: root, name: "gh", args: []string{"pr", "view", "plugins/example", "--json", "url,state,headRefName"}},
		{dir: root, name: "gh", args: []string{"pr", "create", "--head", "plugins/example", "--fill"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestGitCLIPublisherCleansUpWithBackgroundContextAndJoinsErrors(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	ctx, cancel := context.WithCancel(context.Background())
	runner := &recordingRunner{
		results: []commandResult{
			{stdout: "refs/heads/plugins/example\n"},
			{}, {}, {}, {}, {},
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
				{}, {}, {}, {},
				{err: errors.New("add failed")},
				{}, {},
			},
		},
		{
			name: "git push failure",
			results: []commandResult{
				{stdout: "refs/heads/plugins/example\n"},
				{}, {}, {}, {}, {}, {},
				{err: errors.New("push failed")},
				{}, {},
			},
		},
		{
			name: "gh failure",
			results: []commandResult{
				{stdout: "refs/heads/plugins/example\n"},
				{}, {}, {}, {}, {}, {}, {},
				{stderr: "failed to create pr\n", err: errors.New("gh failed")},
				{stdout: "refs/heads/plugins/example\n"},
				{stderr: "no pull requests found", err: errors.New("exit 1")},
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
		{}, {}, {},
		{err: errors.New("worktree add failed")},
		{},
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
		{dir: root, name: "git", args: []string{"status", "--porcelain"}},
		{dir: root, name: "git", args: []string{"ls-remote", "--heads", "origin", "refs/heads/plugins/example"}},
		{dir: root, name: "git", args: []string{"branch", "--", "plugins/example", "HEAD"}},
		{dir: root, name: "git", args: []string{"worktree", "add", worktree, "plugins/example"}},
		{dir: root, name: "git", args: []string{"worktree", "remove", "--force", worktree}},
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

func TestUnavailablePublisherRejectsPublish(t *testing.T) {
	_, err := UnavailablePublisher{}.Publish(context.Background(), PublishRequest{
		Branch:  "plugin/install-demo",
		Message: "install",
		Files:   map[string][]byte{"backend/internal/plugins/demo/plugin.go": []byte("package demo")},
	})
	if !errors.Is(err, domain.ErrPluginPublisherUnavailable) {
		t.Fatalf("Publish() error = %v", err)
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
