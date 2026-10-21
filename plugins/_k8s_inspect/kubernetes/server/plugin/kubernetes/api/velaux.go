package api

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type VelauxApi struct{}

// @Tags VelauxApi
// @Summary 分页获取VelauxApi列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/velaux/:cluster_id/*path [Any]
func (v *VelauxApi) VelauxAPIProxy(c *gin.Context) {
	// 解析参数
	var proxy kubernetesReq.VelauxRequest
	_ = c.ShouldBindQuery(&proxy)
	var urlParam kubernetesReq.ProxyParamRequest
	_ = c.BindUri(&urlParam)

	//参数校验
	if err := sutils.Verify(urlParam, utils.ProxyVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	ret, err := velauxService.Option(c, proxy, urlParam)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败: "+err.Error(), c)
	} else {
		response.OkWithDetailed(ret, "获取成功", c)
	}

	return
}
