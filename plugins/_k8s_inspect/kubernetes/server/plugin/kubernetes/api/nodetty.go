package api

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type NodeTTYApi struct{}

// @Tags NodeTTyApi
// @Summary  NodeTTY 终端资源创建
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/nodetty/get [POST]
func (n *NodeTTYApi) NodeTTYGet(c *gin.Context) {
	var nodetty kubernetesReq.NodeTTY
	_ = c.ShouldBindJSON(&nodetty)

	podMsg, err := nodettyService.Get(nodetty, sutils.GetUserUuid(c))
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败: "+err.Error(), c)
	} else {
		response.OkWithDetailed(podMsg, "获取成功", c)
	}
}
