package proxy

import "strings"

// CompatibleAPIPath mirrors the legacy GVA compatibility rewrites for older/newer clusters.
func CompatibleAPIPath(minor int, apiPath string) string {
	if minor <= 18 && strings.Contains(apiPath, "networking.k8s.io/v1") && strings.Contains(apiPath, "ingresses") {
		return strings.ReplaceAll(apiPath, "networking.k8s.io/v1", "networking.k8s.io/v1beta1")
	}
	if minor > 18 && strings.Contains(apiPath, "/apis/batch/v1beta1") && strings.Contains(apiPath, "cronjobs") {
		return strings.ReplaceAll(apiPath, "/apis/batch/v1beta1", "/apis/batch/v1")
	}
	return apiPath
}
