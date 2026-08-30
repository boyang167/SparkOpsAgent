package service

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/kubernetes"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/podtool"
	"github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"io"
	clientkubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PodsService struct{}

// 全局变量，用于保存UUID与stopChan的映射关系
var (
	portForwardMap  = make(map[string]chan struct{})
	portToLocalPort = make(map[string]int)
	mapMutex        sync.Mutex
)

//@function: ListFiles
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesRequest, uuid uuid.UUID
//@return: files []podtool.File, err error

func (p *PodsService) ListFiles(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (files []podtool.File, err error) {
	pt, err := p.GetPodTool(req, Useruuid)
	if err != nil {
		return nil, err
	}

	return pt.ListFiles(req.Path)

}

//@function: DownloadFile
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesRequest, uuid uuid.UUID
//@return: file string, err error

func (p *PodsService) DownloadFile(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (file string, err error) {
	var fileP string
	pt, err := p.GetPodTool(req, Useruuid)
	if err != nil {
		return fileP, err
	}
	fileNameWithSuffix := path.Base(req.Path)
	fileP = filepath.Join(os.TempDir(), fmt.Sprintf("%d", time.Now().UnixNano()))
	err = os.MkdirAll(fileP, os.ModePerm)
	if err != nil {
		return "", err
	}
	fileP = filepath.Join(fileP, fileNameWithSuffix)
	err = pt.CopyFileFromPod(req.Path, fileP)
	if err != nil {
		return "", err
	}
	return fileP, nil
}

//@function: UploadFile
//@description: 上传文件
//@param: req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID
//@return: err error

func (p *PodsService) UploadFile(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (err error) {
	pt, err := p.GetPodTool(req, Useruuid)
	if err != nil {
		return err
	}
	reader, writer := io.Pipe()
	pt.ExecConfig.Stdin = reader
	go func() {
		defer func() {
			_ = writer.Close()
		}()
		tarFile, err := os.Open(req.FilePath)
		if err != nil {
			logrus.Error(err)
			return
		}
		_, err = io.Copy(writer, tarFile)
		if err != nil {
			logrus.Error(err)
		}
	}()
	return pt.CopyToContainer(req.Path)
}

//@function: DownloadFile
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesRequest
//@return: file string, err error

func (p *PodsService) DeleteFile(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (err error) {
	req.Commands = []string{"rm", "-rf", req.Path}
	pt, err := p.GetPodTool(req, Useruuid)
	if err != nil {
		return err
	}

	_, err = pt.ExecCommand(req.Commands)
	if err != nil {
		return err
	}

	return
}

//@function: Execute
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesRequest
//@return: file string, err error

func (p *PodsService) Execute(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (err error) {
	pt, err := p.GetPodTool(req, Useruuid)
	if err != nil {
		return err
	}

	_, err = pt.ExecCommand(req.Commands)
	if err != nil {
		return err
	}

	return
}

//@function: ExecuteResult
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesRequest
//@return: file string, err error

func (p *PodsService) ExecuteResult(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (err error) {
	pt, err := p.GetPodTool(req, Useruuid)
	if err != nil {
		return err
	}

	output, err := pt.ExecCommand(req.Commands)
	if err != nil {
		return err
	}

	if len(output) < 1 {
		return fmt.Errorf("listening does not exist, please wait")
	}

	return err
}

//@function: ExecuteResult
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesRequest
//@return: file string, err error

func (p *PodsService) ExecuteResultOut(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (ds interface{}, err error) {
	pt, err := p.GetPodTool(req, Useruuid)
	if err != nil {
		return ds, err
	}

	output, err := pt.ExecCommandStdout(req.Commands)
	if err != nil {
		return ds, err
	}

	scanner := bufio.NewScanner(&output)
	var processes []model.ProcessInfo

	// 逐行读取并解析输出
	for scanner.Scan() {
		line := scanner.Text()

		// 过滤包含 "jps" 的行
		if strings.Contains(line, "jps") {
			continue
		}

		parts := strings.Fields(line)

		if len(parts) >= 2 {
			process := model.ProcessInfo{
				Name:  parts[1],
				Value: parts[0],
			}
			processes = append(processes, process)
		}
	}

	return processes, err
}

//@function: GetPodTool
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesRequest, uuid uuid.UUID
//@return: files []model.File, err error

func (p *PodsService) GetPodTool(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID) (pt podtool.PodTool, err error) {
	var cl model.Cluster
	if err := global.GVA_DB.Where("id = ?", req.ClusterId).First(&cl).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return pt, err
	}

	// 获取集群用户信息
	user, err := clusterService.GetClusterByUserUUID(req.ClusterId, Useruuid)
	if err != nil {
		global.GVA_LOG.Error("get user info failed: " + err.Error())
		return pt, err
	}

	k := kubernetes.NewKubernetes(&cl, &user)
	config, err := k.Config()
	if err != nil {
		return pt, err
	}

	clientSet, err := k.Client()
	if err != nil {
		return pt, err
	}

	pt = podtool.PodTool{
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
		K8sClient:     clientSet,
		RestClient:    config,
		ExecConfig: podtool.ExecConfig{
			Stdin: req.Stdin,
		},
	}

	return pt, nil
}

// 随机获取一个本地端口
func getRandomPort() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(65535-1024) + 1024
}

//@function: startPortForward
//@description: 启动端口转发
//@param: req kubernetesReq.PodsFilesReques
//@return: err error

func (p *PodsService) startPortForward(clientset *clientkubernetes.Clientset, config *rest.Config, namespace string, podName string, remotePort int, stopChan chan struct{}) (localPort int, err error) {
	localPort = getRandomPort()
	url := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		global.GVA_LOG.Error("failed to create spdy round tripper: " + err.Error())
		return localPort, fmt.Errorf("failed to create spdy round tripper: %v", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	readyChan := make(chan struct{})
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, nil, nil)
	if err != nil {
		global.GVA_LOG.Error("failed to create port forwarder: " + err.Error())
		return localPort, fmt.Errorf("failed to create port forwarder: %v", err)
	}

	go func() {
		if err := pf.ForwardPorts(); err != nil {
			global.GVA_LOG.Error("Port forwarding failed: " + err.Error())
		}
	}()

	select {
	case <-readyChan:
		global.GVA_LOG.Info(fmt.Sprintf("Namespace: %s Pod: %s Port forwarding started on port %d -> %d.", namespace, podName, localPort, remotePort))
		return localPort, nil
	case <-time.After(10 * time.Second):
		global.GVA_LOG.Error("timeout waiting for port-forward to start")
		return 0, fmt.Errorf("timeout waiting for port-forward to start")
	}
}

//@function: PortForward
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesReques
//@return: err error

func (p *PodsService) PortForward(req kubernetesReq.PodsFilesRequest, Useruuid uuid.UUID, port int) (err error) {
	uniqUUid := uuid.New().String()

	var cl model.Cluster
	if err := global.GVA_DB.Where("id = ?", req.ClusterId).First(&cl).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 获取集群用户信息
	_, err = clusterService.GetClusterByUserUUID(req.ClusterId, Useruuid)
	if err != nil {
		global.GVA_LOG.Error("get user info failed: " + err.Error())
		return err
	}

	k := kubernetes.NewKubernetes(&cl, &model.User{})
	config, err := k.Config()
	if err != nil {
		return err
	}

	clientSet, err := k.Client()
	if err != nil {
		return err
	}

	// 创建停止通道
	stopChan := make(chan struct{})

	// 启动端口转发
	localPort, err := p.startPortForward(clientSet, config, req.Namespace, req.PodName, 8563, stopChan)
	if err != nil {
		global.GVA_LOG.Error("start port forward  Failed: " + err.Error())
		return err
	}

	// 将UUID与stopChan存储在全局map中
	mapMutex.Lock()
	portForwardMap[uniqUUid] = stopChan
	portToLocalPort[uniqUUid] = localPort
	mapMutex.Unlock()

	// 将信息存储到数据库
	portForwardInfo := model.Diagnosis{
		ClusterId:  req.ClusterId,
		UUID:       uniqUUid,
		PodName:    req.PodName,
		Namespace:  req.Namespace,
		LocalPort:  localPort,
		RemotePort: port,
		Status:     1, // 状态设置为活动
	}

	// 检查是否已存在该端口的转发记录
	if errors.Is(global.GVA_DB.Where("cluster_id = ? AND pod_name = ? AND namespace = ? AND remote_port = ?", req.ClusterId, req.PodName, req.Namespace, port).First(&model.Diagnosis{}).Error, gorm.ErrRecordNotFound) {
		// 不存在该端口的记录，存储新记录
		if err := global.GVA_DB.Create(&portForwardInfo).Error; err != nil {
			global.GVA_LOG.Error("failed to save port forward info: " + err.Error())
			return err
		}
	} else {
		global.GVA_LOG.Info(fmt.Sprintf("Port forwarding record already exists for port: %d", port))
	}

	return err
}

//@function: executeCommand
//@description: 根据id获取
//@param: req kubernetesReq.PodsFilesReques
//@return: err error

func (p *PodsService) ExecuteCommand(c *gin.Context, terminal kubernetesReq.TerminalRequest, user *request.CustomClaims, cmd []string) {
	client, err := utils.NewKubeClient(terminal.ClusterId)
	if err != nil {
		response.FailWithMessage(fmt.Sprintf("连接k8s client失败!，%v", err.Error()), c)
		return
	}

	kubeshell, err := utils.NewKubeShell(c.Writer, c.Request, nil)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	exitChan := make(chan bool)

	// WebSocket超时退出逻辑
	go func() {
		for {
			select {
			case <-exitChan:
				global.GVA_LOG.Error("终端记录已经写入退出循环.")
				return
			case <-time.After(time.Second * 2):
				if time.Now().Unix()-kubeshell.UpdatedAt.Unix() > 120 || time.Now().Unix()-kubeshell.WriteAt.Unix() > 120 {
					if _, err = kubeshell.Write([]byte("exit\n")); err != nil {
						global.GVA_LOG.Error("终端WebSocket 终端退出失败：" + err.Error())
					}

					if err := kubeshell.WriteLog(terminal, user); err != nil {
						global.GVA_LOG.Error("终端记录写入失败：" + err.Error())
					}

					if err = kubeshell.Close(); err != nil {
						global.GVA_LOG.Error("终端WebSocket超时关闭失败：" + err.Error())
					}

					global.GVA_LOG.Info("终端WebSocket超时关闭成功！")
					return
				}
			}
		}
	}()

	if err := client.Pod.Exec(cmd, kubeshell, terminal); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	if err := kubeshell.WriteLog(terminal, user); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	exitChan <- true
}
