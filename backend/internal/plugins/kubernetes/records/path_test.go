package records

import "testing"

func TestObjectPathFromPod(t *testing.T) {
	t.Parallel()
	if got := ObjectPathFromPod("default", "web-0", "app"); got != "namespaces/default/pods/web-0/containers/app" {
		t.Fatalf("unexpected path: %q", got)
	}
	if got := ObjectPathFromPod("default", "web-0", ""); got != "namespaces/default/pods/web-0" {
		t.Fatalf("unexpected path without container: %q", got)
	}
	if got := ObjectPathFromPod("", "web-0", "app"); got != "" {
		t.Fatalf("expected empty path: %q", got)
	}
}
