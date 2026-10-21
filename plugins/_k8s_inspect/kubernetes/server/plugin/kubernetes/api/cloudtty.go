package api

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CloudTTYApi struct{}

// @Tags CloudTTYApi
// @Summary  CloudTTY 终端资源创建
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/cloudtty/get [POST]
func (t *CloudTTYApi) CloudTTYGet(c *gin.Context) {
	var tty kubernetesReq.CloudTTY
	_ = c.ShouldBindJSON(&tty)

	podMsg, err := cloudttyService.Get(tty, sutils.GetUserUuid(c))
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败: "+err.Error(), c)
	} else {
		response.OkWithDetailed(podMsg, "获取成功", c)
	}
}
