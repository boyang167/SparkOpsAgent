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
