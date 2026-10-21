package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/kubernetes"
	"github.com/google/uuid"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type ClusterService struct{}

//@function: GetClustersInfoList
//@description: 分页获取列表
//@param: info request.PageInfo
//@return: list interface{}, total int64, err error

func (cls *ClusterService) GetClustersInfoList(cluster model.Cluster, info request.PageInfo, order string, desc bool) (list interface{}, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&model.Cluster{}).Select("id, name, prometheus_url")
	var clsList []model.Cluster

	if cluster.Name != "" {
		db = db.Where("name LIKE ?", "%"+cluster.Name+"%")
	}

	err = db.Count(&total).Error

	if err != nil {
		return clsList, total, err
	} else {
		db = db.Limit(limit).Offset(offset)
		if order != "" {
			var OrderStr string
			// 设置有效排序key 防止sql注入
			orderMap := make(map[string]bool, 5)
			orderMap["id"] = true
			orderMap["name"] = true
			if orderMap[order] {
				if desc {
					OrderStr = order + " desc"
				} else {
					OrderStr = order
				}
			} else { // didn't matched any order key in `orderMap`
				err = fmt.Errorf("非法的排序字段: %v", order)
				return clsList, total, err
			}

			err = db.Order(OrderStr).Find(&clsList).Error
		} else {
			err = db.Order("id").Find(&clsList).Error
		}
	}

	return clsList, total, err
}

//@function: GetGlusterById
//@description: 根据id获取
//@param: id float64
//@return: cluster kubernetes.Cluster, err error

func (cls *ClusterService) GetClusterById(id int) (cluster model.Cluster, err error) {
	err = global.GVA_DB.Where("id = ?", id).Preload("Users").First(&cluster).Error
	return
}

//@function: GetClusterByUserUUID
//@description: 根据id获取
//@param: id float64
//@return: cluster kubernetes.Cluster, err error

func (cls *ClusterService) GetClusterByUserUUID(id int, uuid uuid.UUID) (user model.User, err error) {
	err = global.GVA_DB.Where("cluster_id = ? and uuid = ?", id, uuid).First(&user).Error
	return
}

//@function: CreateCluster
//@description: 新增基础cluster
//@param: cluster kubernetes.Cluster
//@return: err error

func (cls *ClusterService) CreateCluster(cluster model.Cluster) (err error) {
	if !errors.Is(global.GVA_DB.Where("name = ?", cluster.Name).First(&model.Cluster{}).Error, gorm.ErrRecordNotFound) {
		return errors.New("存在相同集群")
	}

	client := kubernetes.NewKubernetes(&cluster, &model.User{})
	if err := client.CheckPermissions(); err != nil {
		global.GVA_LOG.Error("permissions filed: " + err.Error())
		return err
	}

	cluster.UUID = uuid.New()
	if err = global.GVA_DB.Create(&cluster).Error; err != nil {
		return err
	}

	// 异步获取 ca.crt 存储入库
	go func(cluster model.Cluster) {
		caCrt, err := client.GetKubeCaCrt()
		if err != nil {
			global.GVA_LOG.Error("get Ca.crt failed: " + err.Error())
		}

		if err = global.GVA_DB.Model(&model.Cluster{}).Where("id = ?", cluster.ID).Update("kube_ca_crt", caCrt).Error; err != nil {
			global.GVA_LOG.Error("update cluster filed: " + err.Error())
		}
	}(cluster)

	// 异步创建集群默认角色
	go func() {
		if err := client.CreateDefaultClusterRoles(); err != nil {
			global.GVA_LOG.Error("create default cluster roles filed: " + err.Error())
		}
	}()

	return
}

//@author: Eagle
//@function: UpdateCluster
//@description: 根据id更新Cluster
//@param: cluster kubernetes.Cluster
//@return: err error

func (cls *ClusterService) UpdateCluster(cluster model.Cluster) (err error) {
	return global.GVA_DB.Where("id = ?", cluster.ID).First(&model.Cluster{}).Updates(&cluster).Error
}

//@author: Eagle
//@function: DeleteCluster
//@description: 删除cluster
//@param: cluster kubernetes.Cluster
//@return: err error

func (cls *ClusterService) DeleteCluster(req request.GetById) (err error) {
	var cl model.Cluster
	if err = global.GVA_DB.Where("id = ?", req.ID).First(&cl).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// delete  all rbac resource
	client := kubernetes.NewKubernetes(&cl, &model.User{})
	_ = client.CleanAllRBACResource()

	if err = global.GVA_DB.Delete(&cl).Error; err != nil {
		return err
	}

	return err
}

//@author: Eagle
//@function: DeleteClustersByIds
//@description: 删除选中集群
//@param: ids request.IdsReq
//@return: err error

func (cls *ClusterService) DeleteClustersByIds(ids request.IdsReq) (err error) {
	for _, id := range ids.Ids {
		go func(id int) {
			var cl model.Cluster
			if err = global.GVA_DB.Where("id = ?", id).First(&cl).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				global.GVA_LOG.Error("search  cluster filed: " + err.Error())
			}

			// delete  all rbac resource
			client := kubernetes.NewKubernetes(&cl, &model.User{})
			_ = client.CleanAllRBACResource()

			if err = global.GVA_DB.Delete(&cl).Error; err != nil {
				global.GVA_LOG.Error("delete cluster filed: " + err.Error())
			}
		}(id)
	}
	return
}

//@author: Eagle
//@function: CreateCredential
//@description: 集群用户创建凭据
//@param: cluster_id int, user_id uint
//@return: err error

func (cls *ClusterService) CreateCredential(clusterId int, userId uint) (err error) {
	var user system.SysUser
	if err := global.GVA_DB.Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}

	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", clusterId).Preload("Users", "uuid = ?", user.UUID).First(&cluster).Error; err != nil {
		return err
	}

	if !errors.Is(global.GVA_DB.Where("cluster_id = ? and uuid = ?", clusterId, user.UUID).First(&model.User{}).Error, gorm.ErrRecordNotFound) {
		return errors.New("该账号集群凭据已存在")
	}

	// CreateClientCertificate
	k := kubernetes.NewKubernetes(&cluster, &model.User{})
	privateKey, publicCert, err := k.CreateClientCertificate(user.Username)
	if err != nil {
		return errors.New("CreateClientCertificate, error:" + err.Error())
	}

	// To JSON
	kubeConfig, err := utils.KubeconfigJson(cluster, user.Username, privateKey, publicCert)
	if err != nil {
		return errors.New("To YAML, error:" + err.Error())
	}

	// save User
	if err = global.GVA_DB.Save(&model.User{
		UUID:       user.UUID,
		Username:   user.Username,
		NickName:   user.NickName,
		KubeConfig: kubeConfig,
		ClusterId:  uint(clusterId),
	}).Error; err != nil {
		return err
	}

	return err
}

//@author: Eagle
//@function: DeleteClustersByIds
//@description: 删除选中集群
//@param: ids request.IdsReq
//@return: err error

func (cls *ClusterService) GetClusterUserById(clusterId int, userId uint) (user []model.User, err error) {
	var userObj system.SysUser
	if err := global.GVA_DB.Where("id = ?", userId).First(&userObj).Error; err != nil {
		return user, err
	}

	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", clusterId).Preload("Users", "uuid = ?", userObj.UUID).First(&cluster).Error; err != nil {
		return user, err
	}

	if len(cluster.Users) == 0 {
		return nil, errors.New("该集群没有用户")
	}

	if len(cluster.Users) == 1 && cluster.Users[0].UUID == userObj.UUID {
		kubeconfig := &model.Kubeconfig{}
		err = json.Unmarshal([]byte(cluster.Users[0].KubeConfig), kubeconfig)
		if err != nil {
			return nil, errors.New("kubeconfig unmarshal failed: " + err.Error())
		}

		// 获取证书有效期
		if kubeconfig.Users[0].User.ClientCertificateData != "" {
			cert, err := utils.ParseCertificateFromBase64(kubeconfig.Users[0].User.ClientCertificateData)
			if err != nil {
				return nil, errors.New("parse certificate failed: " + err.Error())
			}
			cluster.Users[0].NotBefore = cert.NotBefore
			cluster.Users[0].NotAfter = cert.NotAfter
		}

	}

	return cluster.Users, err
}

//@author: Eagle
//@function: GetClusterRoles
//@description: 获取集群角色信息
//@param: ids request.IdsReq
//@return: err error

func (cls *ClusterService) GetClusterRoles(roleType kubernetesReq.ClusterRoleType) (roles []v1.ClusterRole, err error) {
	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", roleType.ClusterId).First(&cluster).Error; err != nil {
		return roles, err
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})

	return k.ClusterRoles(roleType.RoleType)
}

//@author: Eagle
//@function: GetClusterRoles
//@description: 获取集群角色信息
//@param: ids request.IdsReq
//@return: err error

func (cls *ClusterService) GetClusterApiGroups(apiGroups kubernetesReq.ClusterApiGroups) (groups []model.ApiGroupOption, err error) {
	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", apiGroups.ClusterId).First(&cluster).Error; err != nil {
		return nil, err
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})
	groups, err = k.ServerGroupsAndResources(apiGroups.ApiType)
	if err != nil {
		global.GVA_LOG.Error("ServerGroupsAndResources get resources failed:" + err.Error())
		return nil, err
	}

	return groups, err
}

//@author: Eagle
//@function: CreateClusterRole
//@description: 创建集群角色信息
//@param: role model.RoleData
//@return: err error

func (cls *ClusterService) CreateClusterRole(role model.RoleData) (err error) {
	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", role.ClusterId).First(&cluster).Error; err != nil {
		return err
	}

	for i := range role.Rules {
		for j := range role.Rules[i].APIGroups {
			if role.Rules[i].APIGroups[j] == "core" {
				role.Rules[i].APIGroups[j] = ""
			}
		}
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})
	client, err := k.Client()
	if err != nil {
		return err
	}

	role.Annotations["builtin"] = "false"
	role.Annotations["created-at"] = time.Now().Format("2006-01-02 15:04:05")
	role.Labels[kubernetes.LabelManageKey] = "devops"
	role.Labels[kubernetes.LabelClusterId] = cluster.UUID.String()
	_, err = client.RbacV1().ClusterRoles().Create(context.TODO(), &role.ClusterRole, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return err
}

//@author: Eagle
//@function: UpdateClusterRole
//@description: 更新集群角色信息
//@param: role model.RoleData
//@return: err error

func (cls *ClusterService) UpdateClusterRole(role model.RoleData) (err error) {
	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", role.ClusterId).First(&cluster).Error; err != nil {
		return err
	}

	for i := range role.Rules {
		for j := range role.Rules[i].APIGroups {
			if role.Rules[i].APIGroups[j] == "core" {
				role.Rules[i].APIGroups[j] = ""
			}
		}
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})
	client, err := k.Client()
	if err != nil {
		return err
	}

	instance, err := client.RbacV1().ClusterRoles().Get(context.TODO(), role.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	instance.Annotations["description"] = role.Annotations["description"]
	instance.Rules = role.Rules
	_, err = client.RbacV1().ClusterRoles().Update(context.TODO(), instance, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return err
}

//@author: Eagle
//@function: DeleteClusterRole
//@description: 删除集群角色信息
//@param: role model.RoleData
//@return: err error

func (cls *ClusterService) DeleteClusterRole(role model.RoleData) (err error) {
	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", role.ClusterId).First(&cluster).Error; err != nil {
		return err
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})
	client, err := k.Client()
	if err != nil {
		return err
	}

	return client.RbacV1().ClusterRoles().Delete(context.TODO(), role.Name, metav1.DeleteOptions{})
}

// @author: Eagle
// @function: CreateClusterUser
// @description: 集群批量用户授权
// @param: role kubernetesReq.CreateClusterRole
// @return: err error
func (cls *ClusterService) CreateUser(role kubernetesReq.CreateClusterRole) (err error) {
	for _, userUuid := range role.UserUuids {
		go func(u string, r kubernetesReq.CreateClusterRole) {
			var user system.SysUser
			if err = global.GVA_DB.Where("uuid = ?", u).First(&user).Error; err != nil {
				global.GVA_LOG.Warn("User Not Found failed:" + err.Error())
				return
			}

			if !errors.Is(global.GVA_DB.Where("cluster_id = ? and uuid = ?", r.ClusterId, user.UUID).First(&model.User{}).Error, gorm.ErrRecordNotFound) {
				global.GVA_LOG.Warn(fmt.Sprintf("The User: %s cluster credentials for this account already exist", user.NickName))
				return
			}

			if err = cls.CreateClusterUser(r, user); err != nil {
				global.GVA_LOG.Error("CreateClusterUser failed: " + err.Error())
				return
			}

			return
		}(userUuid, role)
	}

	return err
}

//@author: Eagle
//@function: CreateClusterUser
//@description: 集群用户授权
//@param: role kubernetesReq.CreateClusterRole
//@return: err error

func (cls *ClusterService) CreateClusterUser(role kubernetesReq.CreateClusterRole, user system.SysUser) (err error) {
	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", role.ClusterId).First(&cluster).Error; err != nil {
		return err
	}

	if err = cls.CreateCredential(role.ClusterId, user.ID); err != nil {
		return errors.New("为用户创建集群凭据失败" + err.Error())
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})

	var userData model.User
	if err = global.GVA_DB.Where("cluster_id = ? and uuid = ?", role.ClusterId, user.UUID).First(&userData).Error; err != nil {
		return err
	}

	if len(role.ClusterRoles) > 0 {
		clusterRolesString, err := json.Marshal(role.ClusterRoles)
		if err != nil {
			return err
		}

		for _, clusterRole := range role.ClusterRoles {
			if err = k.CreateOrUpdateClusterRoleBinding(clusterRole, user.Username, false); err != nil {
				global.GVA_LOG.Error("CreateOrUpdateClusterRoleBinding failed:" + err.Error())
			}
		}

		if err = global.GVA_DB.Model(&model.User{}).Where("id = ?", userData.ID).Update("cluster_roles", string(clusterRolesString)).Error; err != nil {
			return err
		}
	}

	if len(role.NamespaceRoles) > 0 {
		namespaceRolesString, err := json.Marshal(role.NamespaceRoles)
		if err != nil {
			return err
		}

		for _, nsRole := range role.NamespaceRoles {
			for _, r := range nsRole.Roles {
				if err := k.CreateOrUpdateRolebinding(nsRole.Namespace, r, user.Username, false); err != nil {
					global.GVA_LOG.Error("CreateOrUpdateRolebinding Namespace role failed:" + err.Error())
				}
			}
		}

		if err = global.GVA_DB.Model(&model.User{}).Where("id = ?", userData.ID).Update("namespace_roles", string(namespaceRolesString)).Error; err != nil {
			return err
		}
	}

	return nil
}

//@author: Eagle
//@function: UpdateClusterUser
//@description: 集群用户授权
//@param: role kubernetesReq.CreateClusterRole
//@return: err error

func (cls *ClusterService) UpdateClusterUser(role kubernetesReq.CreateClusterRole) (err error) {
	var user system.SysUser
	if err := global.GVA_DB.Where("uuid = ?", role.UUID).First(&user).Error; err != nil {
		return err
	}

	var userData model.User
	if errors.Is(global.GVA_DB.Where("cluster_id = ? and uuid = ?", role.ClusterId, user.UUID).First(&userData).Error, gorm.ErrRecordNotFound) {
		return errors.New("该账号集群凭据不存在")
	}

	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", role.ClusterId).Preload("Users", "uuid = ?", role.UUID).First(&cluster).Error; err != nil {
		return err
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})

	// 清理clusterrolebinding
	if err := k.CleanManagedClusterRoleBinding(user.Username); err != nil {
		global.GVA_LOG.Error("CleanManagedClusterRoleBinding failed:" + err.Error())
	}

	// 清理Rolebinding
	if err := k.CleanManagedRoleBinding(user.Username); err != nil {
		global.GVA_LOG.Error("CleanManagedRoleBinding failed:" + err.Error())
	}

	// 先同步完成ClusterRoles更新
	if len(role.ClusterRoles) > 0 {
		clusterRolesString, err := json.Marshal(role.ClusterRoles)
		if err != nil {
			return err
		}

		// 循环创建 ClusterRoleBinding，顺序执行，错误返回
		for _, r := range role.ClusterRoles {
			if err := k.CreateOrUpdateClusterRoleBinding(r, user.Username, false); err != nil {
				global.GVA_LOG.Error("CreateOrUpdateClusterRoleBinding failed:" + err.Error())
				return err
			}
		}

		// 更新数据库
		if err := global.GVA_DB.Model(&model.User{}).Where("id = ?", userData.ID).Update("cluster_roles", string(clusterRolesString)).Error; err != nil {
			return err
		}

	} else {
		if err := global.GVA_DB.Model(&model.User{}).Where("id = ?", userData.ID).Update("cluster_roles", "").Error; err != nil {
			return err
		}
	}

	// 同步完成NamespaceRoles更新
	if len(role.NamespaceRoles) > 0 {
		namespaceRolesString, err := json.Marshal(role.NamespaceRoles)
		if err != nil {
			return err
		}

		for _, nsRole := range role.NamespaceRoles {
			for _, roleName := range nsRole.Roles {
				if err := k.CreateOrUpdateRolebinding(nsRole.Namespace, roleName, user.Username, false); err != nil {
					global.GVA_LOG.Error("CreateOrUpdateRolebinding failed:" + err.Error())
					return err
				}
			}
		}

		if err := global.GVA_DB.Model(&model.User{}).Where("id = ?", userData.ID).Update("namespace_roles", string(namespaceRolesString)).Error; err != nil {
			return err
		}

	} else {
		if err := global.GVA_DB.Model(&model.User{}).Where("id = ?", userData.ID).Update("namespace_roles", "").Error; err != nil {
			return err
		}
	}

	return nil
}

//@author: Eagle
//@function: CreateClusterUser
//@description: 集群用户删除
//@param: role kubernetesReq.CreateClusterRole
//@return: err error

func (cls *ClusterService) DeleteClusterUser(role kubernetesReq.DeleteClusterRole) (err error) {
	for _, userUuid := range role.UserUuids {
		var user model.User
		if err := global.GVA_DB.Where("cluster_id = ? and uuid = ?", role.ClusterId, userUuid).First(&user).Error; err != nil {
			global.GVA_LOG.Warn("Cluster Not Found User")
			continue
		}

		if err := global.GVA_DB.Transaction(func(tx *gorm.DB) error {
			var cluster model.Cluster
			if err := global.GVA_DB.Where("id = ?", role.ClusterId).First(&cluster).Error; err != nil {
				global.GVA_LOG.Warn("Cluster Search failed: " + err.Error())
				return err
			}

			// 清理集群角色绑定
			k := kubernetes.NewKubernetes(&cluster, &model.User{})
			if err := k.CleanManagedClusterRoleBinding(user.Username); err != nil {
				return err
			}

			// 清理命名空间角色绑定
			if err := k.CleanManagedRoleBinding(user.Username); err != nil {
				return err
			}

			TxErr := tx.Where("cluster_id = ? and uuid = ?", role.ClusterId, userUuid).Delete(&model.User{}).Error
			if TxErr != nil {
				return TxErr
			}

			// 返回 nil 提交事务
			return nil
		}); err != nil {
			return err
		}

	}

	return err
}

//@author: Eagle
//@function: GetClusterUserNamespace
//@description: 集群用户已授权命名空间获取
//@param: role kubernetesReq.CreateClusterRole
//@return: err error

func (cls *ClusterService) GetClusterUserNamespace(clusterId int, uuid uuid.UUID) (nsList []string, err error) {
	var user model.User
	if err := global.GVA_DB.Where("cluster_id = ? and uuid = ?", clusterId, uuid).First(&user).Error; err != nil {
		return nil, err
	}

	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", clusterId).First(&cluster).Error; err != nil {
		return nil, err
	}

	k := kubernetes.NewKubernetes(&cluster, &user)

	return k.GetUserNamespaceNames(user.Username)
}

//@author: Eagle
//@function: GetClusterUserNamespace
//@description: 集群用户已授权命名空间获取
//@param: role kubernetesReq.CreateClusterRole
//@return: err error

func (cls *ClusterService) GetClusterListNamespace(clusterId int) (nsList []corev1.Namespace, err error) {
	var cluster model.Cluster
	if err := global.GVA_DB.Where("id = ?", clusterId).First(&cluster).Error; err != nil {
		return nil, err
	}

	k := kubernetes.NewKubernetes(&cluster, &model.User{})
	client, err := k.Client()
	if err != nil {
		return nil, err
	}

	ns, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return ns.Items, err

}
