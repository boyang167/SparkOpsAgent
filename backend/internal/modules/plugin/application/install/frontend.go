package install

import "strings"

// AllowedFrontendRel reports whether a ZIP frontend relative path may be
// written under fronted/apps/web for the given plugin id.
func AllowedFrontendRel(pluginID, rel string) bool {
	if pluginID == "" || rel == "" || strings.Contains(rel, "\\") {
		return false
	}
	prefixes := []string{
		"app/(platform)/plugins/" + pluginID + "/",
		"src/modules/plugins/" + pluginID + "/",
		"src/plugins/" + pluginID + "/",
		"app/api/bff/v1/plugins/" + pluginID + "/",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}
