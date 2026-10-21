package request

import "encoding/json"

type DiagnosisRequest struct {
	ClusterId     int              `json:"cluster_id" form:"cluster_id" validate:"required"`
	PodName       string           `json:"podName" form:"podName" validate:"required"`
	ContainerName string           `json:"containerName" form:"containerName"`
	Namespace     string           `json:"namespace" form:"namespace" validate:"required"`
	ProcessId     string           `json:"processId" form:"processId" validate:"required"`
	Body          *json.RawMessage `json:"body" validate:"required"`
}
