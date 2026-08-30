package service

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	kubernetesReq "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model/kubernetes/request"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/chat"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/kubernetes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	kubernetesClient "k8s.io/client-go/kubernetes"
)

type ChatService struct{}

// ChatQuery 处理聊天查询请求
func (cs *ChatService) ChatQuery(c *gin.Context, req kubernetesReq.ChatReq, uuid uuid.UUID) (err error) {
	// 获取集群信息
	cluster, err := clusterService.GetClusterById(req.ClusterId)
	if err != nil {
		global.GVA_LOG.Error("get cluster info failed: " + err.Error())
		return err
	}

	// 解析用户意图
	intent, err := chat.ParseIntentWithAPI(cluster.BaseURL, cluster.ApiKey, cluster.ModelName, req.Question)
	if err != nil {
		global.GVA_LOG.Error("Failed to parse intent: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 验证资源是否受支持
	gvr, ok := chat.ResourcesMap[intent.Resource]
	if !ok {
		global.GVA_LOG.Error("unsupported resource")
		return
	}

	// 获取集群用户信息
	userObj, err := clusterService.GetClusterByUserUUID(req.ClusterId, uuid)
	if err != nil {
		global.GVA_LOG.Error("get user info failed: " + err.Error())
		return err
	}

	// 获取集群配置
	k := kubernetes.NewKubernetes(&cluster, &userObj)

	// 获取 typedClientSet
	typedClientSet, err := k.Client()
	if err != nil {
		return err
	}

	// 获取动态客户端
	dynamicClientSet, err := k.DynamicClient()
	if err != nil {
		return err
	}

	ns := intent.Namespace

	// 根据意图执行相应操作
	switch intent.Action {
	case "list", "get":
		return cs.handleListGetAction(c, intent, dynamicClientSet, gvr, ns, &cluster, req)
	case "logs":
		return cs.handleLogsAction(c, intent, typedClientSet, ns)
	case "yaml":
		return cs.handleGetYamlAction(c, intent, dynamicClientSet, gvr, ns)
	case "namespace-yaml":
		return cs.handleNamespaceYamlAction(c, intent, dynamicClientSet, gvr, ns)
	case "cluster-yaml":
		return cs.handleClusterYamlAction(c, intent, dynamicClientSet, gvr)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported action"})
	}

	return err
}

// handleListGetAction 处理list和get操作
func (cs *ChatService) handleListGetAction(c *gin.Context, intent *chat.Intent, dynamicClientSet dynamic.Interface, gvr schema.GroupVersionResource, ns string, cluster *model.Cluster, req kubernetesReq.ChatReq) error {
	var list *unstructured.UnstructuredList
	var err error

	if ns == "" || ns == "all" {
		// 查询所有命名空间
		list, err = dynamicClientSet.Resource(gvr).List(context.TODO(), metav1.ListOptions{
			FieldSelector: intent.FieldSelector,
			Limit:         intent.Limit,
		})
	} else {
		// 查询指定命名空间
		list, err = dynamicClientSet.Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{
			FieldSelector: intent.FieldSelector,
			Limit:         intent.Limit,
		})
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return err
	}

	// 格式化输出
	formattedOutput := chat.FormatKubernetesOutput(intent.Resource, list)

	// 先以流的形式返回格式化输出
	chat.StreamResponse(c, fmt.Sprintf("=== 详细数据表格 ===\n%s\n\n", formattedOutput), 10)

	// 使用AI生成自然语言总结
	summary, err := chat.SummarizeWithAI(cluster.BaseURL, cluster.ApiKey, cluster.ModelName, formattedOutput, intent.Resource, req.Question)
	if err != nil {
		global.GVA_LOG.Error("Failed to generate AI summary: " + err.Error())
		// 如果AI总结失败，返回错误信息
		chat.StreamResponse(c, "=== AI总结生成失败 ===\n无法生成智能总结，请稍后重试。\n", 10)
	} else {
		// 再以流的形式返回AI总结
		chat.StreamResponse(c, fmt.Sprintf("=== AI智能总结 ===\n%s\n", summary), 10)
	}

	return nil
}

// handleGetYamlAction 处理yaml获取操作
func (cs *ChatService) handleGetYamlAction(c *gin.Context, intent *chat.Intent, dynamicClientSet dynamic.Interface, gvr schema.GroupVersionResource, ns string) error {
	if intent.ResourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource name is required for yaml"})
		return fmt.Errorf("resource name is required for yaml")
	}

	if ns == "" {
		ns = "default"
	}

	// 获取指定资源
	var resource *unstructured.Unstructured
	var err error

	// 对于集群级别的资源（如nodes, clusterroles等），不需要指定namespace
	if intent.Resource == "nodes" || intent.Resource == "clusterroles" || intent.Resource == "clusterrolebindings" || intent.Resource == "storageclasses" || intent.Resource == "persistentvolumes" {
		resource, err = dynamicClientSet.Resource(gvr).Get(context.TODO(), intent.ResourceName, metav1.GetOptions{})
	} else {
		resource, err = dynamicClientSet.Resource(gvr).Namespace(ns).Get(context.TODO(), intent.ResourceName, metav1.GetOptions{})
	}

	if err != nil {
		global.GVA_LOG.Error("Failed to get resource: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get resource: %v", err)})
		return err
	}

	// 清理不需要的字段
	cleanedObject := make(map[string]interface{})
	for k, v := range resource.Object {
		cleanedObject[k] = v
	}

	// 移除managedFields字段
	if metadata, ok := cleanedObject["metadata"].(map[string]interface{}); ok {
		delete(metadata, "managedFields")
	}

	// 将资源转换为YAML格式
	yamlData, err := yaml.Marshal(cleanedObject)
	if err != nil {
		global.GVA_LOG.Error("Failed to marshal to YAML: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to marshal to YAML: %v", err)})
		return err
	}

	// 以流的形式返回YAML内容
	yamlContent := fmt.Sprintf("=== %s/%s YAML 配置 ===\n```yaml\n%s\n```\n", intent.Resource, intent.ResourceName, string(yamlData))
	chat.StreamResponse(c, yamlContent, 50)

	return nil
}

// handleNamespaceYamlAction 处理获取命名空间所有指定类型资源YAML的请求
func (cs *ChatService) handleNamespaceYamlAction(c *gin.Context, intent *chat.Intent, dynamicClientSet dynamic.Interface, gvr schema.GroupVersionResource, ns string) error {
	if ns == "" {
		ns = "default"
	}

	// 获取指定命名空间下的所有资源
	resourceList, err := dynamicClientSet.Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		global.GVA_LOG.Error("Failed to list resources: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list resources: %v", err)})
		return err
	}

	if len(resourceList.Items) == 0 {
		chat.StreamResponse(c, fmt.Sprintf("=== 命名空间 %s 中没有找到 %s 资源 ===\n", ns, intent.Resource), 10)
		return nil
	}

	// 构建响应头
	response := fmt.Sprintf("=== 命名空间 '%s' 中所有 %s 的 YAML 配置 (%d 个资源) ===\n\n", ns, intent.Resource, len(resourceList.Items))
	chat.StreamResponse(c, response, 10)

	// 遍历所有资源并输出YAML
	for i, resource := range resourceList.Items {
		// 清理不需要的字段
		cleanedObject := make(map[string]interface{})
		for k, v := range resource.Object {
			cleanedObject[k] = v
		}

		// 移除managedFields字段
		if metadata, ok := cleanedObject["metadata"].(map[string]interface{}); ok {
			delete(metadata, "managedFields")
		}

		// 转换为YAML格式
		yamlData, err := yaml.Marshal(cleanedObject)
		if err != nil {
			chat.StreamResponse(c, fmt.Sprintf("转换 %s 为 YAML 时出错: %v\n", resource.GetName(), err), 10)
			continue
		}

		// 输出单个资源的YAML
		resourceResponse := fmt.Sprintf("### %d. %s: %s\n\n```yaml\n%s```\n\n", i+1, intent.Resource, resource.GetName(), string(yamlData))
		chat.StreamResponse(c, resourceResponse, 50)
	}

	chat.StreamResponse(c, "---\n\n✅ 所有 YAML 配置获取完成。\n", 10)
	return nil
}

// handleLogsAction 处理logs操作
func (cs *ChatService) handleLogsAction(c *gin.Context, intent *chat.Intent, typedClientSet *kubernetesClient.Clientset, ns string) error {
	global.GVA_LOG.Info(fmt.Sprintf("Processing logs request for resource: %s, podName: %s, namespace: %s", intent.Resource, intent.PodName, intent.Namespace))

	if intent.Resource != "pods" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "logs only supported for pods"})
		return fmt.Errorf("logs only supported for pods")
	}
	if intent.PodName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pod name is required for logs"})
		return fmt.Errorf("pod name is required for logs")
	}
	if ns == "" {
		ns = "default"
	}

	// 获取pod日志
	podLogOpts := corev1.PodLogOptions{
		TailLines: &[]int64{100}[0], // 获取最后100行
	}

	// 如果指定了容器名称，则设置Container字段
	if intent.ContainerName != "" {
		podLogOpts.Container = intent.ContainerName
		global.GVA_LOG.Info(fmt.Sprintf("Getting logs for container %s", intent.ContainerName))
	}

	req := typedClientSet.CoreV1().Pods(ns).GetLogs(intent.PodName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		global.GVA_LOG.Error("Failed to get logs: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get logs: %v", err)})
		return err
	}
	defer podLogs.Close()

	buf, err := io.ReadAll(podLogs)
	if err != nil {
		global.GVA_LOG.Error("Failed to read logs: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read logs: %v", err)})
		return err
	}

	// 以流的形式返回日志内容
	chat.StreamResponse(c, string(buf), 10)

	return nil
}

// handleClusterYamlAction 处理集群级别YAML获取操作
func (cs *ChatService) handleClusterYamlAction(c *gin.Context, intent *chat.Intent, dynamicClientSet dynamic.Interface, gvr schema.GroupVersionResource) error {
	// 获取集群级别的所有指定类型资源
	list, err := dynamicClientSet.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return err
	}

	// 流式响应开始
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Status(http.StatusOK)

	// 发送资源统计信息
	resourceCount := len(list.Items)
	summaryMsg := fmt.Sprintf("=== 集群级别的 %s 资源 ===\n找到 %d 个资源\n\n", intent.Resource, resourceCount)
	chat.StreamResponse(c, summaryMsg, 10)

	// 逐个处理每个资源
	for i, item := range list.Items {
		// 清理 managedFields 字段
		unstructured.RemoveNestedField(item.Object, "metadata", "managedFields")

		// 转换为YAML
		yamlData, err := yaml.Marshal(item.Object)
		if err != nil {
			continue
		}

		// 发送单个资源的YAML
		resourceMsg := fmt.Sprintf("--- 资源 %d: %s ---\n```yaml\n%s```\n\n", i+1, item.GetName(), string(yamlData))
		chat.StreamResponse(c, resourceMsg, 50)
	}

	return nil
}
