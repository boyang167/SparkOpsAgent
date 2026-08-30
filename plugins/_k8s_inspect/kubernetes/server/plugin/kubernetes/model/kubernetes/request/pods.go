package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	"io"
)

type PodsFilesRequest struct {
	ClusterId     int       `json:"cluster_id" form:"cluster_id" validate:"required"`
	Folder        string    `json:"folder" form:"folder"`
	PodName       string    `json:"podName" form:"podName" validate:"required"`
	ContainerName string    `json:"containerName" form:"containerName"`
	Namespace     string    `json:"namespace" form:"namespace" validate:"required"`
	Path          string    `json:"path" form:"path"`
	OldPath       string    `json:"oldPath" form:"oldPath"`
	Commands      []string  `json:"-" form:"Commands"`
	Stdin         io.Reader `json:"-" form:"Stdin"`
	Content       string    `json:"content" form:"content"`
	FilePath      string    `json:"filePath" form:"filePath"`
	XToken        string    `json:"x-token" form:"x-token"`
}

type SearchRecordParams struct {
	model.PodRecord
	request.PageInfo
	OrderKey string `json:"orderKey"` // 排序
	Desc     bool   `json:"desc"`     // 排序方式:升序false(默认)|降序true
}
