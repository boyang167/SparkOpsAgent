package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type VelauxService struct{}

type user struct {
	CreateTime    time.Time `json:"createTime"`
	LastLoginTime time.Time `json:"lastLoginTime"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Disabled      bool      `json:"disabled"`
}

type responseData struct {
	User         user   `json:"user"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// 认证
func (v VelauxService) Auth(clusterObj model.Cluster) (accessToken string, err error) {
	// 创建要发送的JSON数据结构
	jsonData := map[string]interface{}{
		"username": clusterObj.VelauxUser,
		"password": clusterObj.VelauxPwd,
	}

	// 将JSON数据编码为字节数组
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		global.GVA_LOG.Error("json  Marshal failed", zap.Any("err", err))
		return
	}

	// 创建HTTP请求体
	reqBody := bytes.NewBuffer(jsonBytes)
	loginurl := fmt.Sprintf("%s%s", clusterObj.VelauxUrl, "/api/v1/auth/login")

	// 创建请求
	req, err := http.NewRequest("POST", loginurl, reqBody)
	if err != nil {
		global.GVA_LOG.Error("New Request failed", zap.Any("err", err))
		return
	}

	// 创建请求头，设置Content-Type为application/json
	req.Header.Set("Content-Type", "application/json")

	// 发送请求并获取响应
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		global.GVA_LOG.Error("Request Do failed", zap.Any("err", err))
		return
	}

	defer resp.Body.Close()

	// 读取响应的内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		global.GVA_LOG.Error("Read  Data failed", zap.Any("err", err))
		return
	}

	// 请求响应状态码判断
	if resp.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("request failed: %v", resp))
	}

	// 解析响应的JSON数据到结构体
	var userobj responseData
	if err = json.Unmarshal(body, &userobj); err != nil {
		global.GVA_LOG.Error("json Unmarshal failed: " + string(body))
		return
	}

	return userobj.AccessToken, err
}

//@function: GetMetrics
//@description: 普罗米修斯监控数据获取
//@param: requestInfo kubernetesReq.ResourceParamRequest, queryinfo kubernetesReq.ResourceCreateRequest
//@return: err error

func (v VelauxService) Option(c *gin.Context, proxy kubernetesReq.VelauxRequest, urlParam kubernetesReq.ProxyParamRequest) (ret interface{}, err error) {
	// 获取集群信息
	cluster, err := clusterService.GetClusterById(urlParam.ClusterId)
	if err != nil {
		global.GVA_LOG.Error("get cluster info failed: " + err.Error())
		return nil, err
	}

	// 获取认证AccessToken
	var accessToken string
	var maxRetries = 60
	var retryCount = 0
	for retryCount < maxRetries {
		accessToken, err = v.Auth(cluster)
		if err != nil && accessToken == "" {
			retryCount++
			continue
		}
		// 如果accessToken不为空，或者没有错误，跳出循环
		break
	}

	// 创建一个自定义的Transport，用于添加Token验证
	transport := &http.Transport{}
	client := &http.Client{
		Transport: transport,
	}

	//url格式化
	apiUrl, err := url.Parse(fmt.Sprintf("%s%s", cluster.VelauxUrl, urlParam.Path))
	if err != nil {
		global.GVA_LOG.Error("url Parse failed: " + err.Error())
		return ret, err
	}

	params := url.Values{}
	if proxy.Velaql != "" {
		params.Add("velaql", proxy.Velaql)
		apiUrl.RawQuery = params.Encode()
	}

	req, err := http.NewRequest(c.Request.Method, apiUrl.String(), c.Request.Body)
	if err != nil {
		global.GVA_LOG.Error("New Request failed: " + err.Error())
		return nil, err
	}

	// 设置头信息
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json;charset=utf-8")

	//修改头信息
	if c.Request.Method == "PATCH" {
		req.Header.Set("Content-Type", "application/merge-patch+json")
	}

	// 发送请求并获取响应
	resp, err := client.Do(req)
	if err != nil {
		global.GVA_LOG.Error("Request do failed: " + err.Error())
		return nil, err
	}

	defer resp.Body.Close()

	// 请求响应状态码判断
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("request failed: %v", resp))
	}

	// 读取响应的内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		global.GVA_LOG.Error("Read  Data failed", zap.Any("err", err))
		return nil, err
	}

	//解析出数据
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		global.GVA_LOG.Error("jsonData  Unmarshal failed", zap.Any("err", err))
		return nil, err
	}

	return jsonData, err
}
