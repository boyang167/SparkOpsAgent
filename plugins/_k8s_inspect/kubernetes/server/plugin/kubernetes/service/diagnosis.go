package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"net/http"
	"time"
)

type DiagnosisService struct{}

var (
	podsService = ServiceGroupApp.PodsService
)

//@function: Delete
//@description:  删除
//@param: diagnos model.Diagnosis
//@return: err error

func (d *DiagnosisService) Delete(diagnos model.Diagnosis) (err error) {
	return global.GVA_DB.Delete(&diagnos).Error
}

//@function: DeleteFile
//@description:  删除文件
//@param: req kubernetesReq.DiagnosisRequest, Useruuid uuid.UUID
//@return: err error

func (d *DiagnosisService) DeleteFile(req kubernetesReq.DiagnosisRequest, Useruuid uuid.UUID) (err error) {
	// 1.停止Arthas
	podMsg := kubernetesReq.PodsFilesRequest{
		ClusterId:     req.ClusterId,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
		Namespace:     req.Namespace,
	}

	podMsg.Commands = []string{
		"/bin/sh", "-c",
		"java -jar /tmp/arthas-bin/arthas-client.jar 127.0.0.1 3658 -c \"stop\"",
	}

	if err := podsService.Execute(podMsg, Useruuid); err != nil {
		global.GVA_LOG.Error("Stop Arthas  failed: " + err.Error())
	}

	// 2.删除文件
	filesList := []string{"/tmp/logs", "/tmp/arthas-bin", "/arthas-output", "/tmp/arthas-bin.tar"}
	for file := range filesList {
		podFileDelete := kubernetesReq.PodsFilesRequest{
			ClusterId:     req.ClusterId,
			Folder:        "",
			PodName:       req.PodName,
			ContainerName: req.ContainerName,
			Namespace:     req.Namespace,
			Path:          filesList[file],
		}

		if err := podsService.DeleteFile(podFileDelete, Useruuid); err != nil {
			return err
		}
	}

	return nil
}

//@function: DiagnosisInstall
//@description: 安装
//@param: req kubernetesReq.DiagnosisRequest, Useruuid uuid.UUID
//@return: err error

func (d *DiagnosisService) Install(req kubernetesReq.DiagnosisRequest, Useruuid uuid.UUID) (err error) {
	podMsg := kubernetesReq.PodsFilesRequest{
		ClusterId:     req.ClusterId,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
		Namespace:     req.Namespace,
		Path:          "/tmp",
		FilePath:      "./resource/arthas-bin.tar",
		Commands:      []string{},
	}

	// 1. 判断是否是java容器
	podMsg.Commands = []string{"/bin/sh", "-c", "java -version"}
	if err := podsService.Execute(podMsg, Useruuid); err != nil {
		global.GVA_LOG.Error("container does not have a Java environment: " + err.Error())
		return fmt.Errorf("container does not have a Java environment and cannot install Arthas")
	}

	// 2. 上传文件
	if err := podsService.UploadFile(podMsg, Useruuid); err != nil {
		global.GVA_LOG.Error("upload file failed: " + err.Error())
		return err
	}

	// 3. 启动Arthas
	podMsg.Commands = []string{
		"/bin/sh", "-c",
		fmt.Sprintf("nohup java -jar /tmp/arthas-bin/arthas-boot.jar %s --target-ip 0.0.0.0 > /dev/null 2>&1 &", req.ProcessId),
	}

	if err := podsService.Execute(podMsg, Useruuid); err != nil {
		global.GVA_LOG.Error("Start Arthas failed: " + err.Error())
		return err
	}

	// 4. 开启端口转发
	go d.monitorAndForwardPort(podMsg, Useruuid)

	return err
}

//@function: DiagnosisInstall
//@description: 安装
//@param: (podMsg kubernetesReq.PodsFilesRequest, userUUid uuid.UUID
//@return: err error

func (d *DiagnosisService) monitorAndForwardPort(podMsg kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) {
	ticker := time.NewTicker(3 * time.Second) // 每隔 3 秒触发
	defer ticker.Stop()

	attempts, maxAttempts := 0, 60
	port, hexPort := 8563, ":2173"
	forwarded := false

	for {
		select {
		case <-ticker.C:
			attempts++
			if forwarded {
				return // 如果端口已成功转发，则退出循环
			}

			// 检查端口是否监听
			podMsg.Commands = []string{"/bin/sh", "-c", "cat /proc/net/tcp* | grep '" + hexPort + "'"}
			if err := podsService.ExecuteResult(podMsg, Useruuid); err != nil {
				global.GVA_LOG.Info(fmt.Sprintf("detecting whether port %d exists: %v", port, err))
				continue
			}

			// 端口存在，尝试转发
			if err := podsService.PortForward(podMsg, Useruuid, port); err == nil {
				forwarded = true
				global.GVA_LOG.Info(fmt.Sprintf("PortForward started successfully for port %d with UUID: %s", port, Useruuid.String()))
			} else {
				global.GVA_LOG.Error(fmt.Sprintf("Start PortForward for port %d failed: %v", port, err))
			}

			// 如果已成功转发则退出
			if forwarded {
				global.GVA_LOG.Info("Specified port 8563 is forwarded successfully for UUID: " + Useruuid.String())
				return
			}

			// 如果已经探测 60 次，则退出
			if attempts >= maxAttempts {
				global.GVA_LOG.Error("Exceeded the number of detections, giving up after " + fmt.Sprintf("%d attempts", attempts))
				return
			}
		}
	}
}

//@function: DiagnosisUninstall
//@description: 根据id获取
//@param: req kubernetesReq.DiagnosisInstallReques
//@return: err error

func (d *DiagnosisService) Uninstall(req kubernetesReq.DiagnosisRequest, Useruuid uuid.UUID) (err error) {
	var diagnos model.Diagnosis
	if err := global.GVA_DB.Where("cluster_id = ? AND pod_name = ? AND namespace = ?", req.ClusterId, req.PodName, req.Namespace).Find(&diagnos).Error; err != nil {
		return err
	}

	// 1. 内存关闭端口转发
	if err := d.stopPortForwarding(diagnos.UUID); err != nil {
		global.GVA_LOG.Error("Failed to stop port forwarding for UUID " + diagnos.UUID + ": " + err.Error())
	}

	// 2. 删除文件关闭arthas
	if err := d.DeleteFile(req, Useruuid); err != nil {
		global.GVA_LOG.Error("Failed to delete file for UUID " + diagnos.UUID + ": " + err.Error())
		return err
	}

	// 3. 删除数据库记录
	if err := d.Delete(diagnos); err != nil {
		global.GVA_LOG.Error("Failed to delete database record for UUID " + diagnos.UUID + ": " + err.Error())
		return err
	}

	return nil
}

//@function: stopPortForwarding
//@description: 根据id获取
//@param: uniqUUid string
//@return: err error

func (d *DiagnosisService) stopPortForwarding(uniqUUid string) error {
	mapMutex.Lock()
	defer mapMutex.Unlock()

	stopChan, exists := portForwardMap[uniqUUid]
	if !exists {
		return fmt.Errorf("Invalid UUID, no such port forwarding")
	}

	// 删除UUID对应的stopChan和本地端口映射
	delete(portForwardMap, uniqUUid)
	delete(portToLocalPort, uniqUUid)

	// 关闭stopChan，停止端口转发
	close(stopChan)

	// 给点时间确保转发停止
	time.Sleep(2 * time.Second)
	return nil
}

//@function: Get
//@description: 根据id获取
//@param: req kubernetesReq.DiagnosisInstallReques
//@return: err error

func (d *DiagnosisService) Get(req kubernetesReq.DiagnosisRequest) (ds interface{}, err error) {
	var diagnosis model.Diagnosis
	if !errors.Is(global.GVA_DB.Where("cluster_id = ? AND pod_name = ? AND namespace = ? AND remote_port = ?", req.ClusterId, req.PodName, req.Namespace, 8563).First(&diagnosis).Error, gorm.ErrRecordNotFound) {
		return diagnosis, err
	}

	return diagnosis, err
}

//@function: Jps
//@description: 获取java进程信息
//@param: req kubernetesReq.DiagnosisInstallReques
//@return: err error

func (d *DiagnosisService) Jps(req kubernetesReq.DiagnosisRequest, Useruuid uuid.UUID) (ds interface{}, err error) {
	podMsg := kubernetesReq.PodsFilesRequest{
		ClusterId:     req.ClusterId,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
		Namespace:     req.Namespace,
		Commands:      []string{"/bin/sh", "-c", "jps -l"},
	}

	// 判断是否是java容器
	podMsg.Commands = []string{"/bin/sh", "-c", "java -version"}
	if err := podsService.Execute(podMsg, Useruuid); err != nil {
		global.GVA_LOG.Error("container does not have a Java environment: " + err.Error())
		return nil, fmt.Errorf("container does not have a Java environment and cannot install Arthas")
	}

	// 获取进程信息
	podMsg.Commands = []string{"/bin/sh", "-c", "jps -l"}
	ds, err = podsService.ExecuteResultOut(podMsg, Useruuid)
	if err != nil {
		global.GVA_LOG.Error("container does not have a Java environment: " + err.Error())
		return nil, fmt.Errorf("JPS execution failed: " + err.Error())
	}

	return ds, err
}

//@function: Proxy
//@description: 代理
//@param: req kubernetesReq.DiagnosisInstallReques
//@return: err error

func (d *DiagnosisService) Proxy(reqt kubernetesReq.DiagnosisRequest) (ds interface{}, err error) {
	var diagnosis model.Diagnosis
	if errors.Is(global.GVA_DB.Where("cluster_id = ? AND pod_name = ? AND namespace = ? AND remote_port = ?", reqt.ClusterId, reqt.PodName, reqt.Namespace, 8563).First(&diagnosis).Error, gorm.ErrRecordNotFound) {
		return ds, err
	}

	// 构建请求
	url := fmt.Sprintf("http://localhost:%d/api", diagnosis.LocalPort)
	// 创建请求
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(*reqt.Body))
	if err != nil {
		global.GVA_LOG.Error("Failed to create request failed: " + err.Error())
		return ds, err
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求并获取响应
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		global.GVA_LOG.Error("Failed to send request failed: " + err.Error())
		return ds, fmt.Errorf("request interface failed, please reinstall Arthas")
	}

	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		global.GVA_LOG.Error("Failed to read response body: " + err.Error())
		return ds, err
	}

	// 将 JSON 字节数组解码到 interface{} 中
	if err := json.Unmarshal(body, &ds); err != nil {
		global.GVA_LOG.Error("json Unmarshal  failed: " + err.Error())
		return ds, err
	}

	return ds, err
}
