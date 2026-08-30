package proxy

// CompatibleAPIPath mirrors the legacy GVA compatibility rewrites for older/newer clusters.
func CompatibleAPIPath(minor int, apiPath string) string {
	info, ok := parseListPath(apiPath)
	if !ok {
		return apiPath
	}
	segments := append([]string{}, info.segments...)
	if minor <= 18 && info.groupVersion == "networking.k8s.io/v1" && info.resource == "ingresses" {
		segments[2] = "v1beta1"
		return "/" + joinPathSegments(segments)
	}
	if minor > 18 && info.groupVersion == "batch/v1beta1" && info.resource == "cronjobs" {
		segments[2] = "v1"
		return "/" + joinPathSegments(segments)
	}
	return apiPath
}

func isCompatibilityCandidate(apiPath string) bool {
	info, ok := parseListPath(apiPath)
	if !ok {
		return false
	}
	return (info.groupVersion == "networking.k8s.io/v1" && info.resource == "ingresses") ||
		(info.groupVersion == "batch/v1beta1" && info.resource == "cronjobs")
}
