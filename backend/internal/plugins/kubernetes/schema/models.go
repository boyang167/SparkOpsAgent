package schema

import "time"

type ClusterSettings struct {
	ClusterID          string `gorm:"primaryKey;size:36"`
	PrometheusURL      string
	PrometheusUser     string
	PrometheusPassword string
	VelauxURL          string
	VelauxUser         string
	VelauxPassword     string
	LLMBaseURL         string
	LLMAPIKey          string
	LLMModel           string
}

func (ClusterSettings) TableName() string { return "plugin_kubernetes.cluster_settings" }

type Record struct {
	ID          string `gorm:"primaryKey;size:36"`
	WorkspaceID string `gorm:"index;size:36"`
	ClusterID   string `gorm:"size:36"`
	Kind        string
	StartedAt   time.Time
	EndedAt     *time.Time
	Blob        []byte
	ObjectPath  string
}

func (Record) TableName() string { return "plugin_kubernetes.records" }

type Diagnosis struct {
	ID          string `gorm:"primaryKey;size:36"`
	WorkspaceID string `gorm:"size:36"`
	ClusterID   string `gorm:"size:36"`
	Namespace   string
	Pod         string
	Port        int
	ExpiresAt   time.Time
}

func (Diagnosis) TableName() string { return "plugin_kubernetes.diagnosis" }
