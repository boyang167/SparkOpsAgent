package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
)

type SearchClusterParams struct {
	model.Cluster
	request.PageInfo
	OrderKey string `json:"orderKey"` // 排序
	Desc     bool   `json:"desc"`     // 排序方式:升序false(默认)|降序true
}

type ClusterRoleType struct {
	RoleType  string `json:"role_type"`
	ClusterId uint   `json:"cluster_id"`
}

type ClusterApiGroups struct {
	ApiType   string `json:"api_type"`
	ClusterId uint   `json:"cluster_id"`
}

type NamespaceRoles struct {
	Namespace string   `json:"namespace"`
	Roles     []string `json:"roles"`
}

type CreateClusterRole struct {
	ClusterRoles   []string         `json:"cluster_roles"`
	NamespaceRoles []NamespaceRoles `json:"namespace_roles"`
	UserUuids      []string         `json:"user_uuids"`
	model.User
	ClusterId int `json:"cluster_id"`
}

type DeleteClusterRole struct {
	UserUuids []string `json:"user_uuids"`
	ClusterId int      `json:"cluster_id"`
}
