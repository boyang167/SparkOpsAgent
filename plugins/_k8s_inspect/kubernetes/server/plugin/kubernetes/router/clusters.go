package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type ClusterRouter struct{}

func (u *ClusterRouter) InitClusterRouter(Router *gin.RouterGroup) {
	clusterRouter := Router.Group("kubernetes/clusters").Use(middleware.OperationRecord())
	clustersApi := api.ApiGroupApp.ClustersApi
	{
		clusterRouter.POST("create", clustersApi.CreateCluster)                            // 创建
		clusterRouter.POST("getById", clustersApi.GetClusterById)                          // 单个获取
		clusterRouter.PUT("update", clustersApi.UpdateCluster)                             // 更新
		clusterRouter.DELETE("delete", clustersApi.DeleteCluster)                          // 删除
		clusterRouter.DELETE("deleteByIds", clustersApi.DeleteClustersByIds)               // 批量删除
		clusterRouter.POST("createCredential", clustersApi.CreateCredential)               // 凭据创建
		clusterRouter.POST("getUserById", clustersApi.GetClusterUserById)                  // 集群用户获取
		clusterRouter.POST("getClusterRoles", clustersApi.GetClusterRoles)                 // 集群角色获取
		clusterRouter.POST("getClusterApiGroups", clustersApi.GetClusterApiGroups)         // 资源分组获取
		clusterRouter.POST("createClusterRole", clustersApi.CreateClusterRole)             // 创建角色
		clusterRouter.PUT("updateClusterRole", clustersApi.UpdateClusterRole)              // 更新角色
		clusterRouter.DELETE("deleteClusterRole", clustersApi.DeleteClusterRole)           // 删除角色
		clusterRouter.POST("createClusterUser", clustersApi.CreateClusterUser)             //  创建用户授权
		clusterRouter.PUT("updateClusterUser", clustersApi.UpdateClusterUser)              //  更新用户授权
		clusterRouter.DELETE("deleteClusterUser", clustersApi.DeleteClusterUser)           //  删除用户授权
		clusterRouter.POST("getClusterUserNamespace", clustersApi.GetClusterUserNamespace) //  用户获取命名空间
		clusterRouter.POST("getClusterListNamespace", clustersApi.GetClusterListNamespace) //  管理员获取命名空间
		clusterRouter.POST("list", clustersApi.ClustersList)                               // 分页获取列表
	}
}
