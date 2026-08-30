package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/kubernetes"
	uuid "github.com/google/uuid"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CloudTTYService struct{}

type Metadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

type SecretRef struct {
	Name string `json:"name"`
}

type Spec struct {
	ConfigmapName          string    `json:"configmapName"`
	CommandAction          string    `json:"commandAction"`
	SecretRef              SecretRef `json:"secretRef"`
	Cleanup                bool      `json:"cleanup"`
	Image                  string    `json:"image"`
	TTLSecondsAfterStarted int       `json:"ttlSecondsAfterStarted"`
}

type CloudShell struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

type PodMessage struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Container string `json:"container"`
}

// 随机生成字符串
func randomString() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

//@function: Secret
//@description: 获取CloudTTYService
//@param: user model.User, cluster model.Cluster
//@return: secret []byte, err error

func (t *CloudTTYService) getSecret(user model.User, cluster model.Cluster) (secret []byte, kubeconfigName string, err error) {
	// 用户kubeconfig 凭据 JSON 转 Go 数据结构
	var jsonObj map[string]interface{}
	if err = json.Unmarshal([]byte(user.KubeConfig), &jsonObj); err != nil {
		global.GVA_LOG.Error("Error unmarshalling JSON: " + err.Error())
		return secret, kubeconfigName, err
	}

	// 用户kubeconfig 数据结构转 YAML
	yamlData, err := yaml.Marshal(jsonObj)
	if err != nil {
		global.GVA_LOG.Error("Error marshalling to YAML: " + err.Error())
		return secret, kubeconfigName, err
	}

	// 实例化secret
	kubeconfigName = fmt.Sprintf("%s-kubeconfig-%s", user.Username, cluster.UUID)
	configMap := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigName,
			Namespace: "default",
			Labels: map[string]string{
				"devops/cluster-id": cluster.UUID.String(),
				"devops/manage":     "devops",
				"devops/username":   user.Username,
			},
		},
		Type: "Opaque",
		StringData: map[string]string{
			"config": string(yamlData),
		},
	}

	body, err := json.Marshal(configMap)
	if err != nil {
		global.GVA_LOG.Error("Error JSON Marshal: " + err.Error())
		return secret, kubeconfigName, err
	}

	return body, kubeconfigName, err
}

//@function: CreateSecret
//@description: 创建 CreateSecret
//@param: cluster model.Cluster, body []byte, httpClient http.Client
//@return: secret []byte, err error

func (t *CloudTTYService) CreateSecret(cluster model.Cluster, body []byte, httpClient *http.Client) (err error) {
	//url格式化
	apiUrl, err := url.Parse(fmt.Sprintf("%s%s", cluster.ApiAddress, "/api/v1/namespaces/default/secrets"))
	if err != nil {
		global.GVA_LOG.Error("url Parse failed: " + err.Error())
		return err
	}

	req, err := http.NewRequest("POST", apiUrl.String(), bytes.NewBuffer(body))
	if err != nil {
		global.GVA_LOG.Error("http request failed: " + err.Error())
		return err
	}

	//发起请求
	_, err = httpClient.Do(req)
	if err != nil {
		global.GVA_LOG.Error("http request do failed: " + err.Error())
		return err
	}

	//取出数据Debug调试开启
	//rawResp, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println(string(rawResp))

	return err
}

//@function: DeleteSecret
//@description: 删除 DeleteSecret
//@param: cluster model.Cluster, body []byte, httpClient http.Client
//@return: secret []byte, err error

func (t *CloudTTYService) DeleteSecret(cluster model.Cluster, secret string, httpClient *http.Client) (err error) {
	//url格式化
	apiUrl, err := url.Parse(fmt.Sprintf("%s%s", cluster.ApiAddress, "/api/v1/namespaces/default/secrets/"+secret))
	if err != nil {
		global.GVA_LOG.Error("url Parse failed: " + err.Error())
		return err
	}

	req, err := http.NewRequest("DELETE", apiUrl.String(), nil)
	if err != nil {
		global.GVA_LOG.Error("http request failed: " + err.Error())
		return err
	}

	//发起请求
	_, err = httpClient.Do(req)
	if err != nil {
		global.GVA_LOG.Error("http request do failed: " + err.Error())
		return err
	}

	//取出数据Debug调试开启
	//rawResp, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println(string(rawResp))

	return err
}

//@function: cloudshell
//@description: 获取cloudshell pod 信息
//@param: user model.User, cluster model.Cluster
//@return: configmap []byte, err error

func (t *CloudTTYService) getCloudshell(user model.User, cluster model.Cluster, kubeconfigName string) (cloudshell []byte, podName string, err error) {
	podName = fmt.Sprintf("%s-%s", user.Username, randomString())
	cloudShellData := CloudShell{
		APIVersion: "cloudshell.cloudtty.io/v1alpha1",
		Kind:       "CloudShell",
		Metadata: Metadata{
			Name:      podName,
			Namespace: "default",
			Labels: map[string]string{
				"devops/cluster-id": cluster.UUID.String(),
				"devops/manage":     "devops",
				"devops/username":   user.Username,
				"devops/podName":    podName,
			},
		},
		Spec: Spec{
			SecretRef:              SecretRef{Name: kubeconfigName},
			CommandAction:          "bash",
			Cleanup:                true,
			TTLSecondsAfterStarted: 1800,
		},
	}

	body, err := json.Marshal(cloudShellData)
	if err != nil {
		global.GVA_LOG.Error("Error JSON Marshal: " + err.Error())
		return cloudshell, podName, err
	}

	return body, podName, err
}

//@function: CreateCloudshell
//@description: 创建 CreateCloudshell
//@param: cluster model.Cluster, body []byte, httpClient http.Client
//@return: configmap []byte, err error

func (t *CloudTTYService) CreateCloudshell(cluster model.Cluster, body []byte, httpClient *http.Client) (err error) {
	//url格式化
	apiUrl, err := url.Parse(fmt.Sprintf("%s%s", cluster.ApiAddress, "/apis/cloudshell.cloudtty.io/v1alpha1/namespaces/default/cloudshells"))
	if err != nil {
		global.GVA_LOG.Error("url Parse failed: " + err.Error())
		return err
	}

	req, err := http.NewRequest("POST", apiUrl.String(), bytes.NewBuffer(body))
	if err != nil {
		global.GVA_LOG.Error("http request failed: " + err.Error())
		return err
	}

	//发起请求
	_, err = httpClient.Do(req)
	if err != nil {
		global.GVA_LOG.Error("http request do failed: " + err.Error())
		return err
	}

	//取出数据Debug调试开启
	//rawResp, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println(string(rawResp))

	return err
}

//@function: Get
//@description: 获取CloudTTYService
//@param: id float64
//@return: cluster kubernetes.Cluster, err error

func (t *CloudTTYService) Get(tty kubernetesReq.CloudTTY, uuid uuid.UUID) (podMsg *PodMessage, err error) {
	// 获取集群信息
	cluster, err := clusterService.GetClusterById(tty.ClusterId)
	if err != nil {
		global.GVA_LOG.Error("get cluster info failed: " + err.Error())
		return podMsg, err
	}

	// 获取集群用户信息
	user, err := clusterService.GetClusterByUserUUID(tty.ClusterId, uuid)
	if err != nil {
		global.GVA_LOG.Error("get user info failed: " + err.Error())
		return podMsg, err
	}

	// 获取Secret 信息
	secret, kubeconfigName, err := t.getSecret(user, cluster)
	if err != nil {
		global.GVA_LOG.Error("get ConfigMap Message failed: " + err.Error())
		return podMsg, err
	}

	// 获取 Cloudshell 信息
	cloudshell, cloudshellName, err := t.getCloudshell(user, cluster, kubeconfigName)
	if err != nil {
		global.GVA_LOG.Error("get cloudshell Message failed: " + err.Error())
		return podMsg, err
	}

	// 生成transport
	ts, err := kubernetes.GenerateTLSTransport(&cluster, &model.User{})
	if err != nil {
		global.GVA_LOG.Error("transport generate failed: " + err.Error())
		return podMsg, err
	}

	// 生成httpClient
	httpClient := &http.Client{Transport: ts}

	// 删除Secret
	if err = t.DeleteSecret(cluster, kubeconfigName, httpClient); err != nil {
		global.GVA_LOG.Error("Delete Secret failed: " + err.Error())
		return podMsg, err
	}

	// 创建Secret
	if err = t.CreateSecret(cluster, secret, httpClient); err != nil {
		global.GVA_LOG.Error("Create Secret failed: " + err.Error())
		return podMsg, err
	}

	// 创建 CreateCloudshell
	if err = t.CreateCloudshell(cluster, cloudshell, httpClient); err != nil {
		global.GVA_LOG.Error("Create Cloudshell failed: " + err.Error())
		return podMsg, err
	}

	return &PodMessage{Name: cloudshellName, Namespace: "default", Container: "web-tty"}, err
}
