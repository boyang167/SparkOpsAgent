package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID             uint           `json:"id" gorm:"not null;unique;primary_key"`
	UUID           uuid.UUID      `json:"uuid" gorm:"comment:用户UUID"`
	Username       string         `json:"userName" gorm:"comment:用户登录名"`
	NickName       string         `json:"nickName" gorm:"default:系统用户;comment:用户昵称"`
	KubeConfig     string         `gorm:"type:longText" json:"kube_config" form:"kube_config" gorm:"comment:kube_config"`
	ClusterRoles   string         `gorm:"type:longText" json:"cluster_roles"`
	NamespaceRoles string         `gorm:"type:longText" json:"namespace_roles"`
	ClusterId      uint           `json:"cluster_id" gorm:"comment:集群ID"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	NotBefore      time.Time      `json:"not_before" gorm:"-"`
	NotAfter       time.Time      `json:"not_after" gorm:"-"`
}

func (User) TableName() string {
	return "k8s_users"
}
