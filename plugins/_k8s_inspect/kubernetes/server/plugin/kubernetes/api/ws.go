package api

import (
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

type WsApi struct{}

// 终端token验证
func GetJWTAuth(token string) (user *request.CustomClaims, err error) {
	j := sutils.NewJWT()
	claims, err := j.ParseToken(token)
	if err != nil {
		global.GVA_LOG.Error("从Gin的Context中获取从jwt解析信息失败, 请检查请求头是否存在x-token且claims是否为规定结构")
	}

	return claims, err
}

// @Tags WsApi
// @Summary  终端
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body kubernetesReq.TerminalRequest true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/pods/terminal [get]
func (w *WsApi) Terminal(c *gin.Context) {
	var terminal kubernetesReq.TerminalRequest
	_ = c.ShouldBindQuery(&terminal)

	user, err := GetJWTAuth(terminal.XToken)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := sutils.Verify(terminal, utils.TerminalVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	cmd := []string{
		"/bin/sh", "-c", fmt.Sprintf("clear;(bash || sh); export LINES=%d ; export COLUMNS=%d;", terminal.Rows, terminal.Cols),
	}

	podsService.ExecuteCommand(c, terminal, user, cmd)
}

// @Tags WsApi
// @Summary  终端
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body kubernetesReq.TerminalRequest true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/pods/arthasWs [get]
func (w *WsApi) ArthasTerminal(c *gin.Context) {
	var terminal kubernetesReq.TerminalRequest
	_ = c.ShouldBindQuery(&terminal)

	user, err := GetJWTAuth(terminal.XToken)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := sutils.Verify(terminal, utils.TerminalVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	cmd := []string{
		"/bin/sh", "-c", fmt.Sprintf("java -jar /tmp/arthas-bin/arthas-client.jar; export LINES=%d ; export COLUMNS=%d", terminal.Rows, terminal.Cols),
	}
	podsService.ExecuteCommand(c, terminal, user, cmd)
}

// @Tags WsApi
// @Summary  终端日志
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body kubernetesReq.TerminalRequest true "根据id获取api"
// @Success 200 {object} response.Response{data=systemRes.SysAPIResponse} "根据id获取api,返回包括api详情"
// @Router  /kubernetes/pods/logs [get]
func (w *WsApi) ContainerLog(c *gin.Context) {
	var terminal kubernetesReq.TerminalRequest
	_ = c.ShouldBindQuery(&terminal)
	if _, err := GetJWTAuth(terminal.XToken); err != nil {
		response.FailWithMessage(err.Error(), c)
	}
	if err := sutils.Verify(terminal, utils.TerminalVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	kubeLogger, err := utils.NewKubeLogger(c.Writer, c.Request, nil)
	if err != nil {
		response.FailWithMessage(fmt.Sprintf("连接终端websocket升级失败!，%v", err.Error()), c)
	}

	client, err := utils.NewKubeClient(terminal.ClusterId)
	if err != nil {
		response.FailWithMessage(fmt.Sprintf("连接k8s client失败!，%v", err.Error()), c)
	}

	client.Pod.ContainerLog(kubeLogger, terminal.Name, terminal.PodName, terminal.Namespace)
}
