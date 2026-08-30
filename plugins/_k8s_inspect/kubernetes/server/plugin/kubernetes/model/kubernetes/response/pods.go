package response

import (
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/podtool"
)

type PodsFilesResponse struct {
	Files []podtool.File `json:"files"`
}
