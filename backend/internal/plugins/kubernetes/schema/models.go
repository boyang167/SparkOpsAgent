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
	WorkspaceID string `gorm:"index:idx_records_workspace_started_at,priority:1;size:36"`
	ClusterID   string `gorm:"size:36"`
	Kind        string
	StartedAt   time.Time `gorm:"index:idx_records_workspace_started_at,priority:2,sort:desc"`
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
	Container   string
	PID         int
	Port        int
	SessionDir  string
	JarPath     string
	Status      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

func (Diagnosis) TableName() string { return "plugin_kubernetes.diagnosis" }
