package git

import "testing"

func TestParseGitProjectPath(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"http://192.168.5.24:8929/cicd/test-repo", "cicd/test-repo"},
		{"http://192.168.5.24:8929/cicd/test-repo.git", "cicd/test-repo"},
		{"ssh://git@192.168.5.24:2224/cicd/test-repo.git", "cicd/test-repo"},
		{"git@192.168.5.24:cicd/test-repo.git", "cicd/test-repo"},
	}
	for _, tc := range tests {
		got, err := ParseGitProjectPath(tc.raw)
		if err != nil {
			t.Fatalf("ParseGitProjectPath(%q): %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("ParseGitProjectPath(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestResolveGitLabAPIBase(t *testing.T) {
	base, err := ResolveGitLabAPIBase("ssh://git@192.168.5.24:2224/cicd/test-repo.git", "http://192.168.5.24:8929")
	if err != nil || base != "http://192.168.5.24:8929" {
		t.Fatalf("service url override: got %q err=%v", base, err)
	}

	base, err = ResolveGitLabAPIBase("http://192.168.5.24:8929/cicd/test-repo", "")
	if err != nil || base != "http://192.168.5.24:8929" {
		t.Fatalf("http git url: got %q err=%v", base, err)
	}

	_, err = ResolveGitLabAPIBase("ssh://git@192.168.5.24:2224/cicd/test-repo.git", "")
	if err == nil {
		t.Fatal("expected error when ssh url without service url")
	}
}
