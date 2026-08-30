package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	rbacV1 "k8s.io/api/rbac/v1"
	"time"
)

type Cluster struct {
	ID                 uint           `json:"id" gorm:"not null;unique;primary_key"`
	UUID               uuid.UUID      `json:"uuid" gorm:"comment:集群UUID"`
	Name               string         `json:"name" form:"name" gorm:"comment:集群名称"`
	KubeType           uint           `json:"kube_type" form:"kube_type" gorm:"comment:凭据类型1:KubeConfig,2:Token"`
	KubeConfig         string         `gorm:"type:longText" json:"kube_config" form:"kube_config" gorm:"comment:kube_config"`
	KubeCaCrt          string         `gorm:"type:longText; comment:ca.crt" json:"kube_ca_crt" form:"kube_ca_crt"`
	ApiAddress         string         `gorm:"type:longText" json:"api_address" form:"api_address" gorm:"comment:api_address"`
	PrometheusUrl      string         `gorm:"type:longText" json:"prometheus_url" form:"prometheus_url" gorm:"comment:prometheus 地址"`
	PrometheusAuthType uint           `json:"prometheus_auth_type" form:"prometheus_auth_type" gorm:"comment: 认证类型"`
	PrometheusUser     string         `gorm:"type:longText" json:"prometheus_user" form:"prometheus_user" gorm:"comment:用户名"`
	PrometheusPwd      string         `gorm:"type:longText" json:"prometheus_pwd" form:"prometheus_pwd" gorm:"comment:密码"`
	VelauxUrl          string         `gorm:"type:longText" json:"velaux_url" form:"velaux_url" gorm:"comment:velaux-server地址"`
	VelauxUser         string         `gorm:"type:longText" json:"velaux_user" form:"velaux_user" gorm:"comment:velaux认证用户"`
	VelauxPwd          string         `gorm:"type:longText" json:"velaux_pwd" form:"velaux_pwd" gorm:"comment:velaux认证密码"`
	BaseURL            string         `json:"baseURL" form:"baseURL" gorm:"column:baseURL;comment:;"`
	ApiKey             string         `json:"apiKey" form:"apiKey" gorm:"column:apiKey;comment:;"`
	ModelName          string         `json:"modelName" form:"modelName" gorm:"column:modelName;comment:;"`
	Users              []User         `json:"users" gorm:"foreignKey:ClusterId;"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Cluster) TableName() string {
	return "k8s_clusters"
}

type RoleData struct {
	ClusterId uint `json:"cluster_id"`
	rbacV1.ClusterRole
}
