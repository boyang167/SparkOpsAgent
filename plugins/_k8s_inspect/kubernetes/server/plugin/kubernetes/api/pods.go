package api

import (
	"archive/tar"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	kubernetesRes "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/response"
	sutils "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"
)

type PodsApi struct{}

// @Tags PodsApi
// @Summary 分页获取PodsApi列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.PageInfo true "分页获取API列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "分页获取API列表,返回包括列表,总数,页码,每页数量"
// @Router  /kubernetes/pods/listFiles [post]
func (p *PodsApi) ListFiles(c *gin.Context) {
	var pods kubernetesReq.PodsFilesRequest
	_ = c.ShouldBindJSON(&pods)
	if err := utils.Verify(pods, sutils.PodVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	files, err := podsService.ListFiles(pods, utils.GetUserUuid(c))
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败"+err.Error(), c)
	} else {
		response.OkWithDetailed(kubernetesRes.PodsFilesResponse{Files: files}, "获取成功", c)
	}
}

// @Tags PodsApi
// @Summary 分页获取PodsApi列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.PageInfo true "分页获取API列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "分页获取API列表,返回包括列表,总数,页码,每页数量"
// @Router  /kubernetes/pods/downloadFile [post]
func (p *PodsApi) DownloadFile(c *gin.Context) {
	var pods kubernetesReq.PodsFilesRequest
	_ = c.ShouldBindQuery(&pods)

	user, err := GetJWTAuth(pods.XToken)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
	}

	if err := utils.Verify(pods, sutils.PodVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	file, err := podsService.DownloadFile(pods, user.UUID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败"+err.Error(), c)
	} else {
		c.Header("Content-Disposition", "attachment; filename="+path.Base(file))
		c.Header("Content-Type", "application/octet-stream")
		c.File(file)
	}
}

// @Tags PodsApi
// @Summary 分页获取PodsApi列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.PageInfo true "分页获取API列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "分页获取API列表,返回包括列表,总数,页码,每页数量"
// @Router  /kubernetes/pods/uploadFile [post]
func (p *PodsApi) UploadFile(c *gin.Context) {
	var pods kubernetesReq.PodsFilesRequest
	_ = c.ShouldBindQuery(&pods)
	if err := utils.Verify(pods, sutils.PodVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	srcPath := filepath.Join(os.TempDir(), fmt.Sprintf("%d", time.Now().UnixNano()))
	err := saveTarFile(c, srcPath)
	if err != nil {
		global.GVA_LOG.Error("上传失败!", zap.Error(err))
		response.FailWithMessage("上传失败"+err.Error(), c)
		return
	}

	pods.FilePath = srcPath
	err = podsService.UploadFile(pods, utils.GetUserUuid(c))
	if err != nil {
		global.GVA_LOG.Error("上传失败!", zap.Error(err))
		response.FailWithMessage("上传失败"+err.Error(), c)
	} else {
		response.OkWithMessage("上传成功", c)

	}
}

// @Tags PodsApi
// @Summary 分页获取PodsApi列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body request.PageInfo true "分页获取API列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "分页获取API列表,返回包括列表,总数,页码,每页数量"
// @Router  /kubernetes/pods/deleteFile [post]
func (p *PodsApi) DeleteFile(c *gin.Context) {
	var pods kubernetesReq.PodsFilesRequest
	_ = c.ShouldBindJSON(&pods)
	fmt.Println("pods", pods)
	if err := utils.Verify(pods, sutils.PodVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err := podsService.DeleteFile(pods, utils.GetUserUuid(c))
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败"+err.Error(), c)
	} else {
		response.OkWithMessage("删除成功", c)

	}
}

func saveTarFile(c *gin.Context, srcPath string) error {
	form, err := c.MultipartForm()
	if err != nil {
		global.GVA_LOG.Error("接收文件失败!", zap.Error(err))
		response.FailWithMessage("接收文件失败", c)
	}

	files := form.File["file"]
	fw, err := os.Create(srcPath)
	defer fw.Close()
	if err != nil {
		return err
	}

	tw := tar.NewWriter(fw)
	defer tw.Close()

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.Filename,
			Mode: 0644,
			Size: f.Size,
		}
		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}
		_f, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, _f)
		if err != nil {
			return err
		}
	}

	return nil
}
