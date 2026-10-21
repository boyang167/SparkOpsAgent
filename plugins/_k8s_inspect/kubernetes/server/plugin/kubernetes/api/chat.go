package api

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

type ChatApi struct{}

// @Tags ChatApi
// @Summary Ai助手
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.IdsReq true "ID"
// @Success 200 {object} response.Response{msg=string} "问答结果"
// @Router  /kubernetes/chat/query [POST]
func (ca *ChatApi) ChatQuery(c *gin.Context) {
	var req kubernetesReq.ChatReq
	_ = c.ShouldBindJSON(&req)
	if err := chatServiceService.ChatQuery(c, req, sutils.GetUserUuid(c)); err != nil {
		response.FailWithMessage("问答失败", c)
	}
}
