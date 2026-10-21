package proxy

const gatewayAPIGroup = "gateway.networking.k8s.io"

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

func gatewayVersionCandidates(preferredVersion string) []string {
	switch preferredVersion {
	case "v1":
		return []string{"v1", "v1beta1"}
	case "v1alpha2":
		return []string{"v1alpha2", "v1alpha1"}
	default:
		return []string{preferredVersion}
	}
}

func parseGatewayAPIPath(apiPath string) (group, version, resource string, ok bool) {
	segments := pathSegments(apiPath)
	if len(segments) < 4 || segments[0] != "apis" || segments[1] != gatewayAPIGroup {
		return "", "", "", false
	}
	version = segments[2]
	if len(segments) >= 6 && segments[3] == "namespaces" {
		resource = segments[5]
	} else {
		resource = segments[3]
	}
	if resource == "" || resource == "namespaces" {
		return "", "", "", false
	}
	return gatewayAPIGroup, version, resource, true
}

func rewriteGatewayAPIVersion(apiPath, resolvedVersion string) string {
	segments := pathSegments(apiPath)
	if len(segments) < 4 || segments[0] != "apis" || segments[1] != gatewayAPIGroup {
		return apiPath
	}
	segments[2] = resolvedVersion
	return "/" + joinPathSegments(segments)
}

func isCompatibilityCandidate(apiPath string) bool {
	info, ok := parseListPath(apiPath)
	if !ok {
		return false
	}
	return (info.groupVersion == "networking.k8s.io/v1" && info.resource == "ingresses") ||
		(info.groupVersion == "batch/v1beta1" && info.resource == "cronjobs")
}
