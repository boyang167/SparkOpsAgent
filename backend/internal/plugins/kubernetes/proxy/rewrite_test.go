package proxy

import "testing"

func TestCompatibleAPIPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		minor int
		path  string
		want  string
	}{
		{
			name:  "rewrites networking ingresses for old clusters",
			minor: 18,
			path:  "/apis/networking.k8s.io/v1/ingresses",
			want:  "/apis/networking.k8s.io/v1beta1/ingresses",
		},
		{
			name:  "rewrites batch cronjobs for modern clusters",
			minor: 19,
			path:  "/apis/batch/v1beta1/cronjobs",
			want:  "/apis/batch/v1/cronjobs",
		},
		{
			name:  "leaves unrelated paths unchanged",
			minor: 19,
			path:  "/api/v1/pods",
			want:  "/api/v1/pods",
		},
		{
			name:  "does not rewrite api version substrings",
			minor: 18,
			path:  "/apis/networking.k8s.io/v1alpha1/ingresses",
			want:  "/apis/networking.k8s.io/v1alpha1/ingresses",
		},
		{
			name:  "does not rewrite resource name substrings",
			minor: 18,
			path:  "/apis/networking.k8s.io/v1/not-ingresses",
			want:  "/apis/networking.k8s.io/v1/not-ingresses",
		},
		{
			name:  "rewrites nested namespaced ingress list",
			minor: 18,
			path:  "/apis/networking.k8s.io/v1/namespaces/default/ingresses",
			want:  "/apis/networking.k8s.io/v1beta1/namespaces/default/ingresses",
		},
		{
			name:  "does not rewrite cronjob resource name substrings",
			minor: 19,
			path:  "/apis/batch/v1beta1/cronjobs-old",
			want:  "/apis/batch/v1beta1/cronjobs-old",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := CompatibleAPIPath(tt.minor, tt.path); got != tt.want {
				t.Fatalf("CompatibleAPIPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseGatewayAPIPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		version  string
		resource string
		ok       bool
	}{
		{
			path:     "/apis/gateway.networking.k8s.io/v1/gatewayclasses",
			version:  "v1",
			resource: "gatewayclasses",
			ok:       true,
		},
		{
			path:     "/apis/gateway.networking.k8s.io/v1/namespaces/default/httproutes",
			version:  "v1",
			resource: "httproutes",
			ok:       true,
		},
		{
			path:     "/apis/gateway.networking.k8s.io/v1/namespaces/default/httproutes/demo",
			version:  "v1",
			resource: "httproutes",
			ok:       true,
		},
		{
			path: "/api/v1/pods",
			ok:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			_, version, resource, ok := parseGatewayAPIPath(tt.path)
			if ok != tt.ok {
				t.Fatalf("parseGatewayAPIPath() ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if version != tt.version || resource != tt.resource {
				t.Fatalf("parseGatewayAPIPath() = (%q, %q), want (%q, %q)", version, resource, tt.version, tt.resource)
			}
		})
	}
}

func TestRewriteGatewayAPIVersion(t *testing.T) {
	t.Parallel()

	path := "/apis/gateway.networking.k8s.io/v1/namespaces/default/httproutes/demo"
	want := "/apis/gateway.networking.k8s.io/v1beta1/namespaces/default/httproutes/demo"
	if got := rewriteGatewayAPIVersion(path, "v1beta1"); got != want {
		t.Fatalf("rewriteGatewayAPIVersion() = %q, want %q", got, want)
	}
}

func TestGatewayVersionCandidates(t *testing.T) {
	t.Parallel()

	if got := gatewayVersionCandidates("v1"); len(got) != 2 || got[1] != "v1beta1" {
		t.Fatalf("gatewayVersionCandidates(v1) = %#v", got)
	}
	if got := gatewayVersionCandidates("v1alpha2"); len(got) != 2 || got[1] != "v1alpha1" {
		t.Fatalf("gatewayVersionCandidates(v1alpha2) = %#v", got)
	}
}
