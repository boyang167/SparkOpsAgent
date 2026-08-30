package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestGitCLIPublisherUsesIsolatedWorktreeAndCreatesPullRequest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "plugin-worktree")
	runner := &recordingRunner{outputs: []string{"", "", "", "", "https://example.local/pr/42\n", ""}}
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
		{dir: root, name: "git", args: []string{"worktree", "add", worktree, "-b", "plugins/example"}},
		{dir: worktree, name: "git", args: []string{"add", "--all"}},
		{dir: worktree, name: "git", args: []string{"commit", "-m", "Install example plugin"}},
		{dir: worktree, name: "git", args: []string{"push", "-u", "origin", "plugins/example"}},
		{dir: worktree, name: "gh", args: []string{"pr", "create", "--fill"}},
		{dir: root, name: "git", args: []string{"worktree", "remove", "--force", worktree}},
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
	runner := &worktreePathRunner{t: t, outputs: []string{"", "", "", "", "https://example.local/pr/42\n", ""}}
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

func TestNewGitCLIPublisherFromEnvRequiresCleanGitRoot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runner := &recordingRunner{outputs: []string{"true\n", ""}}
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
	runner := &recordingRunner{outputs: []string{"true\n", " M dirty.go\n"}}
	if _, err := NewGitCLIPublisherFromEnv(ctx, runner); err == nil {
		t.Fatalf("NewGitCLIPublisherFromEnv() dirty root error = nil")
	}
}

type commandCall struct {
	dir  string
	name string
	args []string
}

type recordingRunner struct {
	calls   []commandCall
	outputs []string
	err     error
}

func (r *recordingRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	_ = ctx
	copiedArgs := append([]string(nil), args...)
	r.calls = append(r.calls, commandCall{dir: dir, name: name, args: copiedArgs})
	if r.err != nil {
		return nil, r.err
	}
	if len(r.outputs) == 0 {
		return nil, nil
	}
	out := r.outputs[0]
	r.outputs = r.outputs[1:]
	return []byte(out), nil
}

type worktreePathRunner struct {
	t       *testing.T
	outputs []string
}

func (r *worktreePathRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	_ = ctx
	_ = dir
	if name == "git" && reflect.DeepEqual(args[:2], []string{"worktree", "add"}) {
		if _, err := os.Stat(args[2]); !errors.Is(err, os.ErrNotExist) {
			r.t.Fatalf("worktree path exists before git worktree add: %v", err)
		}
	}
	if len(r.outputs) == 0 {
		return nil, nil
	}
	out := r.outputs[0]
	r.outputs = r.outputs[1:]
	return []byte(out), nil
}
