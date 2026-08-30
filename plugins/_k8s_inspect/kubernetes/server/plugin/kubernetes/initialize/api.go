package initialize

import (
	"context"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/plugin-tool/utils"
)

func Api(ctx context.Context) {
	entities := []model.SysApi{
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/create", Description: "创建"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/getById", Description: "获取"},
		{ApiGroup: "集群管理", Method: "PUT", Path: "/kubernetes/clusters/update", Description: "更新"},
		{ApiGroup: "集群管理", Method: "DELETE", Path: "/kubernetes/clusters/delete", Description: "删除"},
		{ApiGroup: "集群管理", Method: "DELETE", Path: "/kubernetes/clusters/deleteByIds", Description: "批量删除"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/createCredential", Description: "创建凭据"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/getUserById", Description: "用户获取"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/getClusterRoles", Description: "角色获取"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/getClusterApiGroups", Description: "ApiGroups获取"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/createClusterRole", Description: "创建角色"},
		{ApiGroup: "集群管理", Method: "PUT", Path: "/kubernetes/clusters/updateClusterRole", Description: "更新角色"},
		{ApiGroup: "集群管理", Method: "DELETE", Path: "/kubernetes/clusters/deleteClusterRole", Description: "删除角色"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/createClusterUser", Description: "创建用户授权"},
		{ApiGroup: "集群管理", Method: "PUT", Path: "/kubernetes/clusters/updateClusterUser", Description: "更新用户授权"},
		{ApiGroup: "集群管理", Method: "DELETE", Path: "/kubernetes/clusters/deleteClusterUser", Description: "删除用户授权"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/getClusterUserNamespace", Description: "用户获取命名空间"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/getClusterListNamespace", Description: "管理员获取命名空间"},
		{ApiGroup: "集群管理", Method: "POST", Path: "/kubernetes/clusters/list", Description: "分页获取列表"},
		{ApiGroup: "资源列表管理", Method: "POST", Path: "/kubernetes/proxy/*/*", Description: "创建"},
		{ApiGroup: "资源列表管理", Method: "GET", Path: "/kubernetes/proxy/*/*", Description: "获取"},
		{ApiGroup: "资源列表管理", Method: "PUT", Path: "/kubernetes/proxy/*/*", Description: "更新"},
		{ApiGroup: "资源列表管理", Method: "DELETE", Path: "/kubernetes/proxy/*/*", Description: "删除"},
		{ApiGroup: "资源列表管理", Method: "PATCH", Path: "/kubernetes/proxy/*/*", Description: "局部更新"},
		{ApiGroup: "WebSocket", Method: "GET", Path: "/kubernetes/pods/terminal", Description: "终端"},
		{ApiGroup: "WebSocket", Method: "GET", Path: "/kubernetes/pods/logs", Description: "终端日志"},
		{ApiGroup: "普罗米修斯监控", Method: "POST", Path: "/kubernetes/metrics/get", Description: "监控数据"},
		{ApiGroup: "Pods管理", Method: "POST", Path: "/kubernetes/pods/listFiles", Description: "文件列表"},
		{ApiGroup: "Pods管理", Method: "GET", Path: "/kubernetes/pods/downloadFile", Description: "下载文件"},
		{ApiGroup: "Pods管理", Method: "POST", Path: "/kubernetes/pods/uploadFile", Description: "上传文件"},
		{ApiGroup: "Pods管理", Method: "POST", Path: "/kubernetes/pods/deleteFile", Description: "删除文件"},
		{ApiGroup: "终端审计", Method: "POST", Path: "/kubernetes/records/listRecord", Description: "记录列表"},
		{ApiGroup: "终端审计", Method: "POST", Path: "/kubernetes/records/getById", Description: "单个获取"},
		{ApiGroup: "终端审计", Method: "DELETE", Path: "/kubernetes/records/delete", Description: "单个删除"},
		{ApiGroup: "终端审计", Method: "DELETE", Path: "/kubernetes/records/deleteByIds", Description: "批量删除"},
		{ApiGroup: "多集群管理", Method: "POST", Path: "/kubernetes/velaux/*/*", Description: "创建"},
		{ApiGroup: "多集群管理", Method: "GET", Path: "/kubernetes/velaux/*/*", Description: "获取"},
		{ApiGroup: "多集群管理", Method: "PUT", Path: "/kubernetes/velaux/*/*", Description: "更新"},
		{ApiGroup: "多集群管理", Method: "DELETE", Path: "/kubernetes/velaux/*/*", Description: "删除"},
		{ApiGroup: "CloudTTY", Method: "POST", Path: "/kubernetes/cloudtty/get", Description: "获取"},
		{ApiGroup: "NodeTTY", Method: "POST", Path: "/kubernetes/nodetty/get", Description: "获取"},
		{ApiGroup: "应用诊断", Method: "POST", Path: "/kubernetes/diagnosis/install", Description: "安装"},
		{ApiGroup: "应用诊断", Method: "DELETE", Path: "/kubernetes/diagnosis/uninstall", Description: "卸载"},
		{ApiGroup: "应用诊断", Method: "POST", Path: "/kubernetes/diagnosis/getByPod", Description: "获取"},
		{ApiGroup: "应用诊断", Method: "POST", Path: "/kubernetes/diagnosis/proxy", Description: "代理"},
		{ApiGroup: "应用诊断", Method: "POST", Path: "/kubernetes/diagnosis/jps", Description: "进程"},
		{ApiGroup: "应用诊断", Method: "GET", Path: "/kubernetes/pods/arthasWs", Description: "终端"},
		{ApiGroup: "Ai助手", Method: "POST", Path: "/kubernetes/chat/query", Description: "问答"},
	}
	utils.RegisterApis(entities...)
}
