package rbac

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	ManagedNamespace = "sparkops"
	LabelManaged     = "sparkops.agent/managed"
	LabelMemberID    = "sparkops.agent/member-id"
	LabelWorkspaceID = "sparkops.agent/workspace-id"
	LabelClusterID   = "sparkops.agent/cluster-id"
)

func ServiceAccountName(memberID string) string {
	compact := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(memberID)), "-", "")
	if len(compact) > 20 {
		compact = compact[:20]
	}
	return "sparkops-" + compact
}

func ClusterRoleBindingName(memberID, role string) string {
	return bindingName("crb", memberID, "", role)
}

func RoleBindingName(memberID, namespace, role string) string {
	return bindingName("rb", memberID, namespace, role)
}

func bindingName(kind, memberID, namespace, role string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{kind, memberID, namespace, role}, "/")))
	digest := hex.EncodeToString(sum[:])[:12]
	compact := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(memberID)), "-", "")
	if len(compact) > 8 {
		compact = compact[:8]
	}
	name := fmt.Sprintf("sparkops-%s-%s-%s", kind, compact, digest)
	if len(name) > 63 {
		return name[:63]
	}
	return name
}

func managedLabels(workspaceID, clusterID, memberID string) map[string]string {
	return map[string]string{
		LabelManaged:     "true",
		LabelMemberID:    memberID,
		LabelWorkspaceID: workspaceID,
		LabelClusterID:   clusterID,
	}
}

func managedSelector(workspaceID, clusterID, memberID string) string {
	return fmt.Sprintf("%s=true,%s=%s,%s=%s,%s=%s",
		LabelManaged, LabelMemberID, memberID, LabelWorkspaceID, workspaceID, LabelClusterID, clusterID)
}
