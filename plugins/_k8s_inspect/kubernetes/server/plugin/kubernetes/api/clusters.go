package api

import (
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	kubernetesRes "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/response"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ClustersApi struct{}

// @Tags ClustersApi
// @Summary 分页获取ClustersApi列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.PageInfo true "分页获取API列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "分页获取API列表,返回包括列表,总数,页码,每页数量"
// @Router  /kubernetes/clusters/list [post]
func (cl *ClustersApi) ClustersList(c *gin.Context) {
	var pageInfo kubernetesReq.SearchClusterParams
	_ = c.ShouldBindJSON(&pageInfo)
	if err := utils.Verify(pageInfo.PageInfo, utils.PageInfoVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	customerList, total, err := clusterService.GetClustersInfoList(pageInfo.Cluster, pageInfo.PageInfo, pageInfo.OrderKey, pageInfo.Desc)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败"+err.Error(), c)
	} else {
		response.OkWithDetailed(response.PageResult{
			List:     customerList,
			Total:    total,
			Page:     pageInfo.Page,
			PageSize: pageInfo.PageSize,
		}, "获取成功", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/getById [post]
func (cl *ClustersApi) GetClusterById(c *gin.Context) {
	var idInfo request.GetById
	_ = c.ShouldBindJSON(&idInfo)
	if err := utils.Verify(idInfo, utils.IdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	cluster, err := clusterService.GetClusterById(idInfo.ID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
	} else {
		response.OkWithDetailed(kubernetesRes.ClusterResponse{Cluster: cluster}, "获取成功", c)
	}
}

// @Tags ClustersApi
// @Summary 创建基础Cluster
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body kubernetes.Cluster true "创建集群信息"
// @Success 200 {object} response.Response{msg=string} "创建集群信息"
// @Router  /kubernetes/clusters/create [post]
func (cl *ClustersApi) CreateCluster(c *gin.Context) {
	var cluster model.Cluster
	_ = c.ShouldBindJSON(&cluster)
	if err := clusterService.CreateCluster(cluster); err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败: "+err.Error(), c)
	} else {
		response.OkWithMessage("创建成功", c)
	}
}

// @Tags ClustersApi
// @Summary 修改基础ClustersApi
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body  kubernetes.Cluster true "更新"
// @Success 200 {object} response.Response{msg=string} "更新"
// @Router  /kubernetes/clusters/update [put]
func (cl *ClustersApi) UpdateCluster(c *gin.Context) {
	var cluster model.Cluster
	_ = c.ShouldBindJSON(&cluster)
	if err := clusterService.UpdateCluster(cluster); err != nil {
		global.GVA_LOG.Error("修改失败!", zap.Error(err))
		response.FailWithMessage("修改失败", c)
	} else {
		response.OkWithMessage("修改成功", c)
	}
}

// @Tags ClustersApi
// @Summary 删除Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body kubernetes.Cluster true "ID"
// @Success 200 {object} response.Response{msg=string} "删除Clusters"
// @Router  /kubernetes/clusters/delete [delete]
func (cl *ClustersApi) DeleteCluster(c *gin.Context) {
	var idInfo request.GetById
	_ = c.ShouldBindJSON(&idInfo)
	if err := utils.Verify(idInfo, utils.IdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := clusterService.DeleteCluster(idInfo); err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
	} else {
		response.OkWithMessage("删除成功", c)
	}
}

// @Tags ClustersApi
// @Summary 删除选中集群
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "ID"
// @Success 200 {object} response.Response{msg=string} "删除选中集群"
// @Router  /kubernetes/clusters/deleteByIds [delete]
func (cl *ClustersApi) DeleteClustersByIds(c *gin.Context) {
	var ids request.IdsReq
	_ = c.ShouldBindJSON(&ids)
	if err := clusterService.DeleteClustersByIds(ids); err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
	} else {
		response.OkWithMessage("删除成功", c)
	}
}

// @Tags ClustersApi
// @Summary 创建集群凭据
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "ID"
// @Success 200 {object} response.Response{msg=string} "创建集群凭据"
// @Router  /kubernetes/clusters/createCredential [delete]
func (cl *ClustersApi) CreateCredential(c *gin.Context) {
	var idInfo request.GetById
	_ = c.ShouldBindJSON(&idInfo)
	if err := utils.Verify(idInfo, utils.IdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := clusterService.CreateCredential(idInfo.ID, utils.GetUserID(c)); err != nil {
		fmt.Println("err", err.Error())
		global.GVA_LOG.Error("凭据创建失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
	} else {
		response.OkWithMessage("凭据创建成功", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/getUserById [post]
func (cl *ClustersApi) GetClusterUserById(c *gin.Context) {
	var idInfo request.GetById
	_ = c.ShouldBindJSON(&idInfo)
	if err := utils.Verify(idInfo, utils.IdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	cluster, err := clusterService.GetClusterUserById(idInfo.ID, utils.GetUserID(c))
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
	} else {
		response.OkWithDetailed(kubernetesRes.ClusterUserResponse{User: cluster}, "获取成功", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/getClusterRoles [post]
func (cl *ClustersApi) GetClusterRoles(c *gin.Context) {
	var roleType kubernetesReq.ClusterRoleType
	_ = c.ShouldBindJSON(&roleType)
	if err := utils.Verify(roleType, sutils.RoleTypeVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	roles, err := clusterService.GetClusterRoles(roleType)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
	} else {
		response.OkWithDetailed(kubernetesRes.RolesResponse{Roles: roles}, "获取成功", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/createClusterRole [post]
func (cl *ClustersApi) CreateClusterRole(c *gin.Context) {
	var role model.RoleData
	_ = c.ShouldBindJSON(&role)
	if err := utils.Verify(role, sutils.RoleTypeVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err := clusterService.CreateClusterRole(role)
	if err != nil {
		global.GVA_LOG.Error("角色创建失败!", zap.Error(err))
		response.FailWithMessage("角色创建失败!", c)
	} else {
		response.OkWithMessage("角色创建成功!", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/updateClusterRole [post]
func (cl *ClustersApi) UpdateClusterRole(c *gin.Context) {
	var role model.RoleData
	_ = c.ShouldBindJSON(&role)
	if err := utils.Verify(role, sutils.RoleTypeVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err := clusterService.UpdateClusterRole(role)
	if err != nil {
		global.GVA_LOG.Error("角色更新失败!", zap.Error(err))
		response.FailWithMessage("角色更新失败!", c)
	} else {
		response.OkWithMessage("角色更新成功!", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/deleteClusterRole [post]
func (cl *ClustersApi) DeleteClusterRole(c *gin.Context) {
	var role model.RoleData
	_ = c.ShouldBindJSON(&role)
	if err := utils.Verify(role, sutils.RoleTypeVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err := clusterService.DeleteClusterRole(role)
	if err != nil {
		global.GVA_LOG.Error("角色删除失败!", zap.Error(err))
		response.FailWithMessage("角色删除失败!", c)
	} else {
		response.OkWithMessage("角色删除成功!", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/getClusterApiGroups [post]
func (cl *ClustersApi) GetClusterApiGroups(c *gin.Context) {
	var apiGroups kubernetesReq.ClusterApiGroups
	_ = c.ShouldBindJSON(&apiGroups)
	if err := utils.Verify(apiGroups, sutils.ApiGroupsVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	groups, err := clusterService.GetClusterApiGroups(apiGroups)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
	} else {
		response.OkWithDetailed(kubernetesRes.ApiGroupResponse{Groups: groups}, "获取成功", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/createClusterUser [post]
func (cl *ClustersApi) CreateClusterUser(c *gin.Context) {
	var role kubernetesReq.CreateClusterRole
	_ = c.ShouldBindJSON(&role)
	if err := utils.Verify(role, sutils.RoleTypeVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err := clusterService.CreateUser(role)
	if err != nil {
		global.GVA_LOG.Error("用户授权失败!", zap.Error(err))
		response.FailWithMessage("用户授权失败!", c)
	} else {
		response.OkWithMessage("用户授权成功!", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/deleteClusterUser [post]
func (cl *ClustersApi) DeleteClusterUser(c *gin.Context) {
	var role kubernetesReq.DeleteClusterRole
	_ = c.ShouldBindJSON(&role)
	if err := utils.Verify(role, sutils.RoleTypeVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err := clusterService.DeleteClusterUser(role)
	if err != nil {
		global.GVA_LOG.Error("用户删除失败!", zap.Error(err))
		response.FailWithMessage("用户删除失败!", c)
	} else {
		response.OkWithMessage("用户删除成功!", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/updateClusterUser [post]
func (cl *ClustersApi) UpdateClusterUser(c *gin.Context) {
	var role kubernetesReq.CreateClusterRole
	_ = c.ShouldBindJSON(&role)
	if err := utils.Verify(role, sutils.RoleTypeVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err := clusterService.UpdateClusterUser(role)
	if err != nil {
		global.GVA_LOG.Error("用户授权更新失败!", zap.Error(err))
		response.FailWithMessage("用户授权更新失败!", c)
	} else {
		response.OkWithMessage("用户授权更新成功!", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/getClusterUserNamespace [post]
func (cl *ClustersApi) GetClusterUserNamespace(c *gin.Context) {
	var idInfo request.GetById
	_ = c.ShouldBindJSON(&idInfo)
	if err := utils.Verify(idInfo, utils.IdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	namespaces, err := clusterService.GetClusterUserNamespace(idInfo.ID, utils.GetUserUuid(c))
	if err != nil {
		global.GVA_LOG.Error("获取命名空间失败!", zap.Error(err))
		response.FailWithMessage("获取命名空间失败", c)
	} else {
		response.OkWithDetailed(kubernetesRes.ClusterUserNamespace{Namespaces: namespaces}, "获取成功", c)
	}
}

// @Tags ClustersApi
// @Summary 根据id获取Clusters
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.GetById true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/clusters/getClusterListNamespace [post]
func (cl *ClustersApi) GetClusterListNamespace(c *gin.Context) {
	var idInfo request.GetById
	_ = c.ShouldBindJSON(&idInfo)
	if err := utils.Verify(idInfo, utils.IdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	namespaces, err := clusterService.GetClusterListNamespace(idInfo.ID)
	if err != nil {
		global.GVA_LOG.Error("获取命名空间失败!", zap.Error(err))
		response.FailWithMessage("获取命名空间失败", c)
	} else {
		response.OkWithDetailed(kubernetesRes.ClusterListNamespace{Namespaces: namespaces}, "获取成功", c)
	}
}
