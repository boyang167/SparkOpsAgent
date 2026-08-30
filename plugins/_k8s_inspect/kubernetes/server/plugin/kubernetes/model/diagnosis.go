package model

import (
	"gorm.io/gorm"
	"time"
)

type Diagnosis struct {
	ID         uint           `json:"id" gorm:"not null;unique;primaryKey"`
	ClusterId  int            `json:"cluster_id" gorm:"comment:集群ID"`
	UUID       string         `json:"uuid" gorm:"type:varchar(36);uniqueIndex;comment:UUID"`
	PodName    string         `json:"pod_name" gorm:"type:text;comment:Pod名称"`
	Namespace  string         `json:"namespace" gorm:"type:text;comment:命名空间"`
	LocalPort  int            `json:"local_port" gorm:"comment:本地监听端口"`
	RemotePort int            `json:"remote_port"  gorm:"comment:容器内监听端口"`
	Status     int            `json:"status" gorm:"comment:状态"` // 状态字段，1: 活动 2:非活动
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Diagnosis) TableName() string {
	return "k8s_diagnosis"
}

type ProcessInfo struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
