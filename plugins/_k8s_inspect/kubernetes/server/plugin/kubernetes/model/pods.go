package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type PodRecord struct {
	ID            uint           `json:"id" gorm:"not null;unique;primary_key"`
	Cluster       string         `json:"cluster" gorm:"comment:集群名称"`
	Namespace     string         `json:"namespace" gorm:"comment:命名空间"`
	PodName       string         `json:"pod_name" gorm:"comment:Pod名称"`
	ContainerName string         `json:"container_name" gorm:"comment:容器名称"`
	UUID          uuid.UUID      `json:"uuid" gorm:"comment:UUID"`
	Username      string         `json:"userName" gorm:"comment:操作用户"`
	NickName      string         `json:"nickName" gorm:"default:系统用户;comment:用户昵称"`
	Records       []byte         `json:"records" gorm:"type:longblob;comment:'操作记录(二进制存储)';size:128"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (PodRecord) TableName() string {
	return "k8s_pod_records"
}
