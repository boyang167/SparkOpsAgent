package response

import (
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
)

type ClusterResponse struct {
	Cluster model.Cluster `json:"cluster"`
}

type ClusterUserResponse struct {
	User []model.User `json:"user"`
}

type RolesResponse struct {
	Roles []v1.ClusterRole `json:"roles"`
}

type ApiGroupResponse struct {
	Groups []model.ApiGroupOption `json:"groups"`
}

type ClusterUserNamespace struct {
	Namespaces []string `json:"namespaces"`
}

type ClusterListNamespace struct {
	Namespaces []corev1.Namespace `json:"namespaces"`
}
