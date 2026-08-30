package api

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	kubernetesRes "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/response"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DiagnosisApi struct{}

// @Tags DiagnosisApi
// @Summary  Diagnosis 安装 arthas
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/diagnosis/install [POST]
func (d *DiagnosisApi) DiagnosisInstall(c *gin.Context) {
	var installreq kubernetesReq.DiagnosisRequest
	_ = c.ShouldBindJSON(&installreq)

	if err := utils.Verify(installreq, sutils.DiagnosisVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := diagnosisService.Install(installreq, utils.GetUserUuid(c)); err != nil {
		global.GVA_LOG.Error("安装失败!", zap.Error(err))
		response.FailWithMessage("安装失败: "+err.Error(), c)
	} else {
		response.OkWithMessage("安装成功", c)
	}
}

// @Tags DiagnosisApi
// @Summary  Diagnosis 安装 arthas
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/diagnosis/uninstall [POST]
func (d *DiagnosisApi) DiagnosisUninstall(c *gin.Context) {
	var installreq kubernetesReq.DiagnosisRequest
	_ = c.ShouldBindJSON(&installreq)

	if err := utils.Verify(installreq, sutils.DiagnosisVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if err := diagnosisService.Uninstall(installreq, utils.GetUserUuid(c)); err != nil {
		global.GVA_LOG.Error("卸载失败!", zap.Error(err))
		response.FailWithMessage("卸载失败: "+err.Error(), c)
	} else {
		response.OkWithMessage("卸载成功", c)
	}
}

// @Tags DiagnosisApi
// @Summary  Diagnosis 安装 arthas
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/diagnosis/getByPod [POST]
func (d *DiagnosisApi) DiagnosisByPod(c *gin.Context) {
	var getreq kubernetesReq.DiagnosisRequest
	_ = c.ShouldBindJSON(&getreq)

	if err := utils.Verify(getreq, sutils.DiagnosisVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	diagnosis, err := diagnosisService.Get(getreq)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败: "+err.Error(), c)
	} else {
		response.OkWithDetailed(kubernetesRes.DiagnosisResponse{Diagnosis: diagnosis}, "获取成功", c)
	}
}

// @Tags DiagnosisApi
// @Summary  Diagnosis 安装 arthas
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/diagnosis/jps [POST]
func (d *DiagnosisApi) DiagnosisJps(c *gin.Context) {
	var getreq kubernetesReq.DiagnosisRequest
	_ = c.ShouldBindJSON(&getreq)

	if err := utils.Verify(getreq, sutils.DiagnosisVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	diagnosis, err := diagnosisService.Jps(getreq, utils.GetUserUuid(c))
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败: "+err.Error(), c)
	} else {
		response.OkWithDetailed(kubernetesRes.DiagnosisResponse{Diagnosis: diagnosis}, "获取成功", c)
	}
}

// @Tags DiagnosisApi
// @Summary  Diagnosis 代理
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Router  /kubernetes/diagnosis/proxy [Any]
func (d *DiagnosisApi) DiagnosisProxy(c *gin.Context) {
	var req kubernetesReq.DiagnosisRequest
	_ = c.ShouldBindJSON(&req)

	if err := utils.Verify(req, sutils.DiagnosisProxyVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	arthasData, err := diagnosisService.Proxy(req)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败: "+err.Error(), c)
	} else {
		response.OkWithDetailed(kubernetesRes.ArthasDataResponse{Arthas: arthasData}, "获取成功", c)
	}
}
