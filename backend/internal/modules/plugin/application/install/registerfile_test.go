package install

import (
	"strings"
	"testing"
)

const testModulePath = "github.com/sparkops/sparkops-agent/backend"

func TestRewriteRegisterGoFromEmptyFile(t *testing.T) {
	got, err := RewriteRegisterGo(nil, testModulePath, []string{"example"})
	if err != nil {
		t.Fatalf("RewriteRegisterGo() error = %v", err)
	}

	want := `package plugins

import _ "github.com/sparkops/sparkops-agent/backend/internal/plugins/example"
`
	if string(got) != want {
		t.Fatalf("RewriteRegisterGo() =\n%s\nwant:\n%s", got, want)
	}
}

func TestRewriteRegisterGoAddsAndRemovesPluginImports(t *testing.T) {
	first, err := RewriteRegisterGo(nil, testModulePath, []string{"example"})
	if err != nil {
		t.Fatalf("RewriteRegisterGo(first) error = %v", err)
	}

	second, err := RewriteRegisterGo(first, testModulePath, []string{"second", "example"})
	if err != nil {
		t.Fatalf("RewriteRegisterGo(second) error = %v", err)
	}
	wantSecond := `package plugins

import (
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/example"
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/second"
)
`
	if string(second) != wantSecond {
		t.Fatalf("RewriteRegisterGo(second) =\n%s\nwant:\n%s", second, wantSecond)
	}

	third, err := RewriteRegisterGo(second, testModulePath, []string{"second"})
	if err != nil {
		t.Fatalf("RewriteRegisterGo(third) error = %v", err)
	}
	wantThird := `package plugins

import _ "github.com/sparkops/sparkops-agent/backend/internal/plugins/second"
`
	if string(third) != wantThird {
		t.Fatalf("RewriteRegisterGo(third) =\n%s\nwant:\n%s", third, wantThird)
	}
}

func TestRewriteRegisterGoSortsDeduplicatesAndKeepsOnlyPluginBlankImports(t *testing.T) {
	existing := []byte(`package other

import (
	"fmt"
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/old"
	named "github.com/sparkops/sparkops-agent/backend/internal/plugins/named"
	_ "example.com/not/a/plugin"
)

var keepMe = fmt.Sprintf("not part of register file")
`)

	got, err := RewriteRegisterGo(existing, testModulePath, []string{"zeta", "alpha", "alpha"})
	if err != nil {
		t.Fatalf("RewriteRegisterGo() error = %v", err)
	}

	want := `package plugins

import (
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/alpha"
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/zeta"
)
`
	if string(got) != want {
		t.Fatalf("RewriteRegisterGo() =\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(string(got), "fmt") || strings.Contains(string(got), "old") || strings.Contains(string(got), "named") {
		t.Fatalf("RewriteRegisterGo() retained non-requested imports:\n%s", got)
	}
}

func TestRewriteRegisterGoWithNoPluginsIsValidPackage(t *testing.T) {
	got, err := RewriteRegisterGo(nil, testModulePath, nil)
	if err != nil {
		t.Fatalf("RewriteRegisterGo() error = %v", err)
	}

	want := "package plugins\n"
	if string(got) != want {
		t.Fatalf("RewriteRegisterGo() = %q, want %q", got, want)
	}
}
