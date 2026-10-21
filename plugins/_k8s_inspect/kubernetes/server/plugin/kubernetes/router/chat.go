package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type ChatApiRouter struct{}

func (c *ChatApiRouter) InitChatRouter(Router *gin.RouterGroup) {
	chatRouter := Router.Group("kubernetes/chat").Use(middleware.OperationRecord())
	chatApi := api.ApiGroupApp.ChatApi
	chatRouter.POST("/query", chatApi.ChatQuery) // Ai助手
}
