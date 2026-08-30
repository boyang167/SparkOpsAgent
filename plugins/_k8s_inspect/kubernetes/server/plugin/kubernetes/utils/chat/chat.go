package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type NLRequest struct {
	Text string `json:"text"`
}

type Intent struct {
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace"`
	FieldSelector string `json:"fieldSelector"`
	Limit         int64  `json:"limit"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
	// 资源创建相关字段
	ResourceName string            `json:"resourceName"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Replicas     int32             `json:"replicas"`
	Image        string            `json:"image"`
	Port         int32             `json:"port"`
	ServiceType  string            `json:"serviceType"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatChoice struct {
	Message ChatMessage `json:"message"`
}

type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

var ResourcesMap = map[string]schema.GroupVersionResource{
	// Core resources
	"pods":                   {Group: "", Version: "v1", Resource: "pods"},
	"nodes":                  {Group: "", Version: "v1", Resource: "nodes"},
	"services":               {Group: "", Version: "v1", Resource: "services"},
	"events":                 {Group: "", Version: "v1", Resource: "events"},
	"namespaces":             {Group: "", Version: "v1", Resource: "namespaces"},
	"configmaps":             {Group: "", Version: "v1", Resource: "configmaps"},
	"secrets":                {Group: "", Version: "v1", Resource: "secrets"},
	"persistentvolumes":      {Group: "", Version: "v1", Resource: "persistentvolumes"},
	"persistentvolumeclaims": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"serviceaccounts":        {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"componentstatuses":      {Group: "", Version: "v1", Resource: "componentstatuses"},
	// Apps resources
	"deployments":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"replicasets":  {Group: "apps", Version: "v1", Resource: "replicasets"},
	"daemonsets":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"statefulsets": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	// Batch resources
	"jobs":     {Group: "batch", Version: "v1", Resource: "jobs"},
	"cronjobs": {Group: "batch", Version: "v1", Resource: "cronjobs"},
	// Networking resources
	"ingresses":       {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"networkpolicies": {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	// RBAC resources
	"roles":               {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	"rolebindings":        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	"clusterroles":        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
	"clusterrolebindings": {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	// Storage resources
	"storageclasses":       {Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
	"volumeattachments":    {Group: "storage.k8s.io", Version: "v1", Resource: "volumeattachments"},
	"csidrivers":           {Group: "storage.k8s.io", Version: "v1", Resource: "csidrivers"},
	"csinodes":             {Group: "storage.k8s.io", Version: "v1", Resource: "csinodes"},
	"csistoragecapacities": {Group: "storage.k8s.io", Version: "v1", Resource: "csistoragecapacities"},
	// API Extensions resources
	"customresourcedefinitions": {Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"},
	// Scheduling resources
	"priorityclasses": {Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
	// Node resources
	"runtimeclasses": {Group: "node.k8s.io", Version: "v1", Resource: "runtimeclasses"},
	// Admission resources
	"mutatingwebhookconfigurations":   {Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingwebhookconfigurations"},
	"validatingwebhookconfigurations": {Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingwebhookconfigurations"},
	// Certificate resources
	"certificatesigningrequests": {Group: "certificates.k8s.io", Version: "v1", Resource: "certificatesigningrequests"},
	// Policy resources
	"podsecuritypolicies":  {Group: "policy", Version: "v1beta1", Resource: "podsecuritypolicies"},
	"poddisruptionbudgets": {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
}

// TableFormatter 表格格式化器，借鉴 Kubernetes 的 tabwriter 实现
type TableFormatter struct {
	headers []string
	rows    [][]string
}

// NewTableFormatter 创建新的表格格式化器
func NewTableFormatter(headers []string) *TableFormatter {
	return &TableFormatter{
		headers: headers,
		rows:    make([][]string, 0),
	}
}

// AddRow 添加数据行
func (tf *TableFormatter) AddRow(row []string) {
	if len(row) != len(tf.headers) {
		return // 列数不匹配，忽略
	}
	tf.rows = append(tf.rows, row)
}

// Format 格式化输出表格，使用固定宽度列实现对齐
func (tf *TableFormatter) Format() string {
	var output strings.Builder

	// 计算每列的最大宽度，设置最小宽度
	colWidths := make([]int, len(tf.headers))
	for i, header := range tf.headers {
		colWidths[i] = len(header)
		// 设置最小列宽为8个字符
		if colWidths[i] < 8 {
			colWidths[i] = 8
		}
	}
	for _, row := range tf.rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// 输出表头，使用固定宽度对齐
	for i, header := range tf.headers {
		if i > 0 {
			output.WriteString("    ") // 四个空格作为列间距
		}
		output.WriteString(fmt.Sprintf("%-*s", colWidths[i], strings.ToUpper(header)))
	}
	output.WriteString("\n")

	// 输出数据行，使用固定宽度对齐
	for _, row := range tf.rows {
		for i, cell := range row {
			if i > 0 {
				output.WriteString("    ") // 四个空格作为列间距
			}
			if i < len(colWidths) {
				output.WriteString(fmt.Sprintf("%-*s", colWidths[i], cell))
			} else {
				output.WriteString(cell)
			}
		}
		output.WriteString("\n")
	}

	return output.String()
}

// 格式化时间
func FormatAge(t time.Time) string {
	duration := time.Since(t)
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd", days)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// 格式化kubernetes输出
func FormatKubernetesOutput(resource string, list *unstructured.UnstructuredList) string {
	if len(list.Items) == 0 {
		return fmt.Sprintf("No %s found.\n", resource)
	}

	switch resource {
	case "pods":
		// 使用 TableFormatter 格式化 pods 输出
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "READY", "STATUS", "RESTARTS", "AGE"})

		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			status := "Unknown"
			ready := "0/0"
			restarts := "0"
			age := "Unknown"
			totalContainers := 0
			readyContainers := 0
			totalRestarts := int64(0)

			if statusObj, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if phase, found, _ := unstructured.NestedString(statusObj, "phase"); found {
					status = phase
				}

				// 获取容器状态信息
				if containerStatuses, found, _ := unstructured.NestedSlice(statusObj, "containerStatuses"); found {
					totalContainers = len(containerStatuses)
					for _, containerStatus := range containerStatuses {
						if containerMap, ok := containerStatus.(map[string]interface{}); ok {
							// 检查容器是否就绪
							if isReady, found, _ := unstructured.NestedBool(containerMap, "ready"); found && isReady {
								readyContainers++
							}
							// 累计重启次数
							if restartCount, found, _ := unstructured.NestedInt64(containerMap, "restartCount"); found {
								totalRestarts += restartCount
							}
						}
					}
				}
			}

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			// 获取容器数量（如果没有从status获取到）
			if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
				if containersSlice, found, _ := unstructured.NestedSlice(spec, "containers"); found {
					if totalContainers == 0 {
						totalContainers = len(containersSlice)
					}
				}
			}

			// 格式化ready和restarts字段
			ready = fmt.Sprintf("%d/%d", readyContainers, totalContainers)
			restarts = fmt.Sprintf("%d", totalRestarts)

			// 添加数据行到表格
			table.AddRow([]string{namespace, name, ready, status, restarts, age})
		}

		return table.Format()
	case "nodes":
		table := NewTableFormatter([]string{"NAME", "STATUS", "ROLES", "AGE", "VERSION"})
		for _, item := range list.Items {
			name := item.GetName()
			status := "Unknown"
			roles := "<none>"
			age := "Unknown"
			version := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			// 解析节点状态
			if statusObj, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				// 获取节点版本信息
				if nodeInfo, found, _ := unstructured.NestedMap(statusObj, "nodeInfo"); found {
					if kubeletVersion, found, _ := unstructured.NestedString(nodeInfo, "kubeletVersion"); found {
						version = kubeletVersion
					}
				}

				// 解析节点状态条件
				if conditions, found, _ := unstructured.NestedSlice(statusObj, "conditions"); found {
					for _, condition := range conditions {
						if conditionMap, ok := condition.(map[string]interface{}); ok {
							if conditionType, found, _ := unstructured.NestedString(conditionMap, "type"); found && conditionType == "Ready" {
								if conditionStatus, found, _ := unstructured.NestedString(conditionMap, "status"); found {
									if conditionStatus == "True" {
										status = "Ready"
									} else {
										status = "NotReady"
									}
								}
								break
							}
						}
					}
				}
			}

			// 解析节点角色
			if metadata, found, _ := unstructured.NestedMap(item.Object, "metadata"); found {
				if labels, found, _ := unstructured.NestedStringMap(metadata, "labels"); found {
					var nodeRoles []string
					for key := range labels {
						if strings.HasPrefix(key, "node-role.kubernetes.io/") {
							role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
							if role != "" {
								nodeRoles = append(nodeRoles, role)
							}
						}
					}
					if len(nodeRoles) > 0 {
						roles = strings.Join(nodeRoles, ",")
					}
				}
			}

			table.AddRow([]string{name, status, roles, age, version})
		}

		return table.Format()
	case "services":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "TYPE", "CLUSTER-IP", "EXTERNAL-IP", "PORT(S)", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			svcType := "Unknown"
			clusterIP := "<none>"
			externalIP := "<none>"
			ports := "<none>"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
				if typeStr, found, _ := unstructured.NestedString(spec, "type"); found {
					svcType = typeStr
				}
				if clusterIPStr, found, _ := unstructured.NestedString(spec, "clusterIP"); found {
					clusterIP = clusterIPStr
				}

				// Parse ports from spec
				if portsSlice, found, _ := unstructured.NestedSlice(spec, "ports"); found && len(portsSlice) > 0 {
					var portList []string
					for _, port := range portsSlice {
						if portMap, ok := port.(map[string]interface{}); ok {
							if portNum, found, _ := unstructured.NestedInt64(portMap, "port"); found {
								if protocol, found, _ := unstructured.NestedString(portMap, "protocol"); found {
									portList = append(portList, fmt.Sprintf("%d/%s", portNum, protocol))
								} else {
									portList = append(portList, fmt.Sprintf("%d", portNum))
								}
							}
						}
					}
					if len(portList) > 0 {
						ports = strings.Join(portList, ",")
					}
				}
			}

			// Parse external IP from status
			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if loadBalancer, found, _ := unstructured.NestedMap(status, "loadBalancer"); found {
					if ingress, found, _ := unstructured.NestedSlice(loadBalancer, "ingress"); found && len(ingress) > 0 {
						var externalIPs []string
						for _, ing := range ingress {
							if ingMap, ok := ing.(map[string]interface{}); ok {
								if ip, found, _ := unstructured.NestedString(ingMap, "ip"); found && ip != "" {
									externalIPs = append(externalIPs, ip)
								} else if hostname, found, _ := unstructured.NestedString(ingMap, "hostname"); found && hostname != "" {
									externalIPs = append(externalIPs, hostname)
								}
							}
						}
						if len(externalIPs) > 0 {
							externalIP = strings.Join(externalIPs, ",")
						}
					}
				}
			}

			table.AddRow([]string{namespace, name, svcType, clusterIP, externalIP, ports, age})
		}

		return table.Format()
	case "deployments":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "READY", "UP-TO-DATE", "AVAILABLE", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			ready := "0/0"
			upToDate := "0"
			available := "0"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if replicas, found, _ := unstructured.NestedInt64(status, "replicas"); found {
					if readyReplicas, found, _ := unstructured.NestedInt64(status, "readyReplicas"); found {
						ready = fmt.Sprintf("%d/%d", readyReplicas, replicas)
					}
				}
				if updatedReplicas, found, _ := unstructured.NestedInt64(status, "updatedReplicas"); found {
					upToDate = fmt.Sprintf("%d", updatedReplicas)
				}
				if availableReplicas, found, _ := unstructured.NestedInt64(status, "availableReplicas"); found {
					available = fmt.Sprintf("%d", availableReplicas)
				}
			}

			table.AddRow([]string{namespace, name, ready, upToDate, available, age})
		}

		return table.Format()
	case "namespaces":
		table := NewTableFormatter([]string{"NAME", "STATUS", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			status := "Unknown"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if statusObj, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if phase, found, _ := unstructured.NestedString(statusObj, "phase"); found {
					status = phase
				}
			}

			table.AddRow([]string{name, status, age})
		}

		return table.Format()
	case "configmaps", "secrets":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "DATA", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			data := "0"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if dataObj, found, _ := unstructured.NestedMap(item.Object, "data"); found {
				data = fmt.Sprintf("%d", len(dataObj))
			}

			table.AddRow([]string{namespace, name, data, age})
		}

		return table.Format()
	case "persistentvolumes":
		table := NewTableFormatter([]string{"NAME", "CAPACITY", "ACCESS MODES", "RECLAIM POLICY", "STATUS", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			capacity := "Unknown"
			accessModes := "Unknown"
			reclaimPolicy := "Unknown"
			status := "Unknown"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			// Parse spec fields
			if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
				// Parse capacity
				if capacityMap, found, _ := unstructured.NestedStringMap(spec, "capacity"); found {
					if storage, exists := capacityMap["storage"]; exists {
						capacity = storage
					}
				}

				// Parse access modes
				if accessModesSlice, found, _ := unstructured.NestedStringSlice(spec, "accessModes"); found && len(accessModesSlice) > 0 {
					accessModes = strings.Join(accessModesSlice, ",")
				}

				// Parse reclaim policy
				if policy, found, _ := unstructured.NestedString(spec, "persistentVolumeReclaimPolicy"); found {
					reclaimPolicy = policy
				}
			}

			// Parse status
			if statusObj, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if phase, found, _ := unstructured.NestedString(statusObj, "phase"); found {
					status = phase
				}
			}

			table.AddRow([]string{name, capacity, accessModes, reclaimPolicy, status, age})
		}

		return table.Format()
	case "persistentvolumeclaims":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "STATUS", "VOLUME", "CAPACITY", "ACCESS MODES", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			status := "Unknown"
			volume := "Unknown"
			capacity := "Unknown"
			accessModes := "Unknown"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			// Parse spec fields
			if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
				// Parse access modes from spec
				if accessModesSlice, found, _ := unstructured.NestedStringSlice(spec, "accessModes"); found && len(accessModesSlice) > 0 {
					accessModes = strings.Join(accessModesSlice, ",")
				}

				// Parse volume name from spec
				if volumeName, found, _ := unstructured.NestedString(spec, "volumeName"); found {
					volume = volumeName
				}
			}

			// Parse status fields
			if statusObj, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				// Parse phase (status)
				if phase, found, _ := unstructured.NestedString(statusObj, "phase"); found {
					status = phase
				}

				// Parse capacity from status
				if capacityMap, found, _ := unstructured.NestedStringMap(statusObj, "capacity"); found {
					if storage, exists := capacityMap["storage"]; exists {
						capacity = storage
					}
				}

				// Parse access modes from status (if not found in spec)
				if accessModes == "Unknown" {
					if accessModesSlice, found, _ := unstructured.NestedStringSlice(statusObj, "accessModes"); found && len(accessModesSlice) > 0 {
						accessModes = strings.Join(accessModesSlice, ",")
					}
				}
			}

			table.AddRow([]string{namespace, name, status, volume, capacity, accessModes, age})
		}

		return table.Format()
	case "ingresses":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "CLASS", "HOSTS", "ADDRESS", "PORTS", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			ingressClass := "<none>"
			hosts := "*"
			address := "<none>"
			ports := "80"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			// Parse ingressClassName from spec
			if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
				if className, found, _ := unstructured.NestedString(spec, "ingressClassName"); found {
					ingressClass = className
				}

				// Parse rules for hosts and ports
				if rules, found, _ := unstructured.NestedSlice(spec, "rules"); found && len(rules) > 0 {
					hostList := []string{}
					portSet := make(map[string]bool)
					for _, rule := range rules {
						if ruleMap, ok := rule.(map[string]interface{}); ok {
							if host, found, _ := unstructured.NestedString(ruleMap, "host"); found && host != "" {
								hostList = append(hostList, host)
							}
							if http, found, _ := unstructured.NestedMap(ruleMap, "http"); found {
								if paths, found, _ := unstructured.NestedSlice(http, "paths"); found {
									for _, path := range paths {
										if pathMap, ok := path.(map[string]interface{}); ok {
											if backend, found, _ := unstructured.NestedMap(pathMap, "backend"); found {
												if service, found, _ := unstructured.NestedMap(backend, "service"); found {
													if port, found, _ := unstructured.NestedMap(service, "port"); found {
														if number, found, _ := unstructured.NestedInt64(port, "number"); found {
															portSet[fmt.Sprintf("%d", number)] = true
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
					if len(hostList) > 0 {
						hosts = strings.Join(hostList, ",")
					}
					if len(portSet) > 0 {
						portList := []string{}
						for port := range portSet {
							portList = append(portList, port)
						}
						ports = strings.Join(portList, ",")
					}
				}
			}

			// Parse loadBalancer status for address
			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if loadBalancer, found, _ := unstructured.NestedMap(status, "loadBalancer"); found {
					if ingress, found, _ := unstructured.NestedSlice(loadBalancer, "ingress"); found && len(ingress) > 0 {
						addressList := []string{}
						for _, ing := range ingress {
							if ingMap, ok := ing.(map[string]interface{}); ok {
								if ip, found, _ := unstructured.NestedString(ingMap, "ip"); found && ip != "" {
									addressList = append(addressList, ip)
								} else if hostname, found, _ := unstructured.NestedString(ingMap, "hostname"); found && hostname != "" {
									addressList = append(addressList, hostname)
								}
							}
						}
						if len(addressList) > 0 {
							address = strings.Join(addressList, ",")
						}
					}
				}
			}

			table.AddRow([]string{namespace, name, ingressClass, hosts, address, ports, age})
		}

		return table.Format()
	case "replicasets":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "DESIRED", "CURRENT", "READY", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			desired := "0"
			current := "0"
			ready := "0"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if replicas, found, _ := unstructured.NestedInt64(status, "replicas"); found {
					current = fmt.Sprintf("%d", replicas)
				}
				if readyReplicas, found, _ := unstructured.NestedInt64(status, "readyReplicas"); found {
					ready = fmt.Sprintf("%d", readyReplicas)
				}
			}

			if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
				if replicas, found, _ := unstructured.NestedInt64(spec, "replicas"); found {
					desired = fmt.Sprintf("%d", replicas)
				}
			}

			table.AddRow([]string{namespace, name, desired, current, ready, age})
		}

		return table.Format()
	case "daemonsets":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "DESIRED", "CURRENT", "READY", "UP-TO-DATE", "AVAILABLE", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			desired := "0"
			current := "0"
			ready := "0"
			upToDate := "0"
			available := "0"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if desiredNumberScheduled, found, _ := unstructured.NestedInt64(status, "desiredNumberScheduled"); found {
					desired = fmt.Sprintf("%d", desiredNumberScheduled)
				}
				if currentNumberScheduled, found, _ := unstructured.NestedInt64(status, "currentNumberScheduled"); found {
					current = fmt.Sprintf("%d", currentNumberScheduled)
				}
				if numberReady, found, _ := unstructured.NestedInt64(status, "numberReady"); found {
					ready = fmt.Sprintf("%d", numberReady)
				}
				if updatedNumberScheduled, found, _ := unstructured.NestedInt64(status, "updatedNumberScheduled"); found {
					upToDate = fmt.Sprintf("%d", updatedNumberScheduled)
				}
				if numberAvailable, found, _ := unstructured.NestedInt64(status, "numberAvailable"); found {
					available = fmt.Sprintf("%d", numberAvailable)
				}
			}

			table.AddRow([]string{namespace, name, desired, current, ready, upToDate, available, age})
		}

		return table.Format()
	case "statefulsets":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "READY", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			ready := "0/0"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if replicas, found, _ := unstructured.NestedInt64(status, "replicas"); found {
					if readyReplicas, found, _ := unstructured.NestedInt64(status, "readyReplicas"); found {
						ready = fmt.Sprintf("%d/%d", readyReplicas, replicas)
					}
				}
			}

			table.AddRow([]string{namespace, name, ready, age})
		}

		return table.Format()
	case "jobs":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "COMPLETIONS", "DURATION", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			completions := "0/1"
			duration := "Unknown"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if succeeded, found, _ := unstructured.NestedInt64(status, "succeeded"); found {
					if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
						if specCompletions, found, _ := unstructured.NestedInt64(spec, "completions"); found {
							completions = fmt.Sprintf("%d/%d", succeeded, specCompletions)
						}
					}
				}
				if startTime, found, _ := unstructured.NestedString(status, "startTime"); found {
					if completionTime, found, _ := unstructured.NestedString(status, "completionTime"); found {
						if start, err := time.Parse(time.RFC3339, startTime); err == nil {
							if end, err := time.Parse(time.RFC3339, completionTime); err == nil {
								duration = end.Sub(start).String()
							}
						}
					}
				}
			}

			table.AddRow([]string{namespace, name, completions, duration, age})
		}

		return table.Format()
	case "cronjobs":
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "SCHEDULE", "SUSPEND", "ACTIVE", "LAST SCHEDULE", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			schedule := "Unknown"
			suspend := "False"
			active := "0"
			lastSchedule := "<none>"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			if spec, found, _ := unstructured.NestedMap(item.Object, "spec"); found {
				if scheduleStr, found, _ := unstructured.NestedString(spec, "schedule"); found {
					schedule = scheduleStr
				}
				if suspendBool, found, _ := unstructured.NestedBool(spec, "suspend"); found && suspendBool {
					suspend = "True"
				}
			}

			if status, found, _ := unstructured.NestedMap(item.Object, "status"); found {
				if activeJobs, found, _ := unstructured.NestedSlice(status, "active"); found {
					active = fmt.Sprintf("%d", len(activeJobs))
				}
				if lastScheduleTime, found, _ := unstructured.NestedString(status, "lastScheduleTime"); found {
					if t, err := time.Parse(time.RFC3339, lastScheduleTime); err == nil {
						lastSchedule = FormatAge(t)
					}
				}
			}

			table.AddRow([]string{namespace, name, schedule, suspend, active, lastSchedule, age})
		}

		return table.Format()
	case "storageclasses":
		table := NewTableFormatter([]string{"NAME", "PROVISIONER", "RECLAIMPOLICY", "VOLUMEBINDINGMODE", "ALLOWVOLUMEEXPANSION", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			provisioner := "<unknown>"
			reclaimPolicy := "Delete"
			volumeBindingMode := "Immediate"
			allowVolumeExpansion := "false"
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			// StorageClass fields are at root level, not in spec
			if prov, found, _ := unstructured.NestedString(item.Object, "provisioner"); found {
				provisioner = prov
			}
			if policy, found, _ := unstructured.NestedString(item.Object, "reclaimPolicy"); found {
				reclaimPolicy = policy
			}
			if bindingMode, found, _ := unstructured.NestedString(item.Object, "volumeBindingMode"); found {
				volumeBindingMode = bindingMode
			}
			if expansion, found, _ := unstructured.NestedBool(item.Object, "allowVolumeExpansion"); found {
				if expansion {
					allowVolumeExpansion = "true"
				}
			}

			table.AddRow([]string{name, provisioner, reclaimPolicy, volumeBindingMode, allowVolumeExpansion, age})
		}

		return table.Format()
	case "componentstatuses":
		table := NewTableFormatter([]string{"NAME", "STATUS", "MESSAGE"})
		for _, item := range list.Items {
			name := item.GetName()
			status := "Unknown"
			message := ""

			// 获取组件状态信息
			if conditions, found, _ := unstructured.NestedSlice(item.Object, "conditions"); found {
				for _, condition := range conditions {
					if conditionMap, ok := condition.(map[string]interface{}); ok {
						if conditionType, found, _ := unstructured.NestedString(conditionMap, "type"); found && conditionType == "Healthy" {
							if conditionStatus, found, _ := unstructured.NestedString(conditionMap, "status"); found {
								if conditionStatus == "True" {
									status = "Healthy"
								} else {
									status = "Unhealthy"
								}
							}
							if conditionMessage, found, _ := unstructured.NestedString(conditionMap, "message"); found {
								message = conditionMessage
							}
							break
						}
					}
				}
			}

			table.AddRow([]string{name, status, message})
		}

		return table.Format()
	case "clusterroles":
		// 使用 TableFormatter 格式化 clusterroles 输出
		table := NewTableFormatter([]string{"NAME", "CREATED AT"})

		for _, item := range list.Items {
			name := item.GetName()
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			table.AddRow([]string{name, age})
		}

		return table.Format()
	case "clusterrolebindings":
		// 使用 TableFormatter 格式化 clusterrolebindings 输出
		table := NewTableFormatter([]string{"NAME", "ROLE", "AGE"})

		for _, item := range list.Items {
			name := item.GetName()
			role := "Unknown"
			age := "Unknown"

			if roleRef, found, _ := unstructured.NestedMap(item.Object, "roleRef"); found {
				if roleName, found, _ := unstructured.NestedString(roleRef, "name"); found {
					role = roleName
				}
			}

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			table.AddRow([]string{name, role, age})
		}

		return table.Format()
	case "customresourcedefinitions":
		// 使用 TableFormatter 格式化 customresourcedefinitions 输出
		table := NewTableFormatter([]string{"NAME", "CREATED AT"})

		for _, item := range list.Items {
			name := item.GetName()
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			table.AddRow([]string{name, age})
		}

		return table.Format()
	case "priorityclasses":
		// 使用 TableFormatter 格式化 priorityclasses 输出
		table := NewTableFormatter([]string{"NAME", "VALUE", "GLOBAL-DEFAULT", "AGE"})

		for _, item := range list.Items {
			name := item.GetName()
			value := "0"
			globalDefault := "false"
			age := "Unknown"

			if val, found, _ := unstructured.NestedInt64(item.Object, "value"); found {
				value = fmt.Sprintf("%d", val)
			}

			if gd, found, _ := unstructured.NestedBool(item.Object, "globalDefault"); found {
				globalDefault = fmt.Sprintf("%t", gd)
			}

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			table.AddRow([]string{name, value, globalDefault, age})
		}

		return table.Format()
	default:
		// 对于其他资源类型，显示基本信息
		table := NewTableFormatter([]string{"NAMESPACE", "NAME", "AGE"})
		for _, item := range list.Items {
			name := item.GetName()
			namespace := item.GetNamespace()
			age := "Unknown"

			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				age = FormatAge(creationTime.Time)
			}

			table.AddRow([]string{namespace, name, age})
		}

		return table.Format()
	}
}

// streamResponse 以OpenAI标准流格式返回响应
func StreamResponse(c *gin.Context, content string, chunkSize int) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Status(http.StatusOK)

	// 检查内容是否包含表格数据，如果是则用代码块包装以保持格式
	if strings.Contains(content, "=== 详细数据表格 ===") {
		// 将表格内容包装在代码块中以保持等宽字体格式
		lines := strings.Split(content, "\n")
		var formattedLines []string
		inTableSection := false

		for _, line := range lines {
			if strings.Contains(line, "=== 详细数据表格 ===") {
				formattedLines = append(formattedLines, line)
				formattedLines = append(formattedLines, "```")
				inTableSection = true
			} else if strings.Contains(line, "=== AI智能总结 ===") {
				if inTableSection {
					formattedLines = append(formattedLines, "```")
					inTableSection = false
				}
				formattedLines = append(formattedLines, "")
				formattedLines = append(formattedLines, line)
			} else {
				formattedLines = append(formattedLines, line)
			}
		}

		// 如果还在表格区域，添加结束标记
		if inTableSection {
			formattedLines = append(formattedLines, "```")
		}

		content = strings.Join(formattedLines, "\n")
	}

	// 将内容转换为rune数组以正确处理UTF-8字符
	runes := []rune(content)
	totalLen := len(runes)

	// 分块发送，遵循OpenAI流格式，按字符而不是字节切分
	for i := 0; i < totalLen; i += chunkSize {
		end := i + chunkSize
		if end > totalLen {
			end = totalLen
		}

		// 将rune切片转换为字符串，确保UTF-8编码正确
		chunkContent := string(runes[i:end])

		// 构造OpenAI流格式的数据
		streamData := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "kubernetes-assistant",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": chunkContent,
					},
					"finish_reason": nil,
				},
			},
		}

		jsonData, _ := json.Marshal(streamData)
		c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", jsonData))
		c.Writer.Flush()

		// 添加小延迟以模拟流式传输
		time.Sleep(50 * time.Millisecond)
	}

	// 发送结束标记
	finishData := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   "kubernetes-assistant",
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": "stop",
			},
		},
	}

	finishJsonData, _ := json.Marshal(finishData)
	c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", finishJsonData))
	c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

// ========= 调用 大模型 API =========
func ParseIntentWithAPI(baseURL, apiKey, modelName, text string) (*Intent, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY not set")
	}

	client := resty.New()
	systemPrompt := `你是一个 Kubernetes 助手。
用户会输入自然语言，你需要输出严格的 JSON，字段有：
{ "action": "list|get|logs|yaml|namespace-yaml|cluster-yaml", "resource": "pods|nodes|services|events|namespaces|configmaps|secrets|persistentvolumes|persistentvolumeclaims|serviceaccounts|componentstatuses|deployments|replicasets|daemonsets|statefulsets|jobs|cronjobs|ingresses|networkpolicies|roles|rolebindings|clusterroles|clusterrolebindings|storageclasses|volumeattachments|csidrivers|csinodes|csistoragecapacities|customresourcedefinitions|priorityclasses|runtimeclasses|mutatingwebhookconfigurations|validatingwebhookconfigurations|certificatesigningrequests|podsecuritypolicies|poddisruptionbudgets", "namespace": "...", "fieldSelector": "...", "limit": 10, "podName": "...", "containerName": "...", "resourceName": "..." }

动作说明：
- list/get: 查询资源
- logs: 查看Pod日志
- yaml: 获取指定资源的YAML文件
- namespace-yaml: 获取指定命名空间中所有指定类型资源的YAML文件
- cluster-yaml: 获取集群级别的所有指定类型资源的YAML文件

当用户要求查看日志时，action应该是"logs"，resource是"pods"，并且需要提取podName。
如果用户指定了容器名称，需要提取containerName字段。如果Pod有多个容器但用户没有指定容器名，containerName可以为空。

当用户要求获取单个资源的YAML文件时，action应该是"yaml"，需要提取resourceName字段指定具体的资源名称。例如："获取pod nginx的yaml"应该解析为{"action":"yaml","resource":"pods","resourceName":"nginx"}。

当用户要求获取某个命名空间的所有资源YAML时，action应该是"namespace-yaml"，需要提取namespace和resource字段。例如："获取default命名空间的所有deployment yaml"应该解析为{"action":"namespace-yaml","resource":"deployments","namespace":"default"}。

当用户要求获取集群级别的所有资源YAML时，action应该是"cluster-yaml"，只需要提取resource字段。例如："获取所有nodes的yaml"或"获取集群中所有clusterroles的yaml"应该解析为{"action":"cluster-yaml","resource":"nodes"}或{"action":"cluster-yaml","resource":"clusterroles"}。集群级别资源包括：nodes、clusterroles、clusterrolebindings、persistentvolumes、storageclasses、componentstatuses、volumeattachments、csidrivers、csinodes、customresourcedefinitions、priorityclasses、runtimeclasses、mutatingwebhookconfigurations、validatingwebhookconfigurations、certificatesigningrequests、podsecuritypolicies、poddisruptionbudgets等。

当用户要求检查集群组件状态时，如"k get cs"、"检查集群组件状态"、"查看集群健康状态"等，应该解析为{"action":"list","resource":"componentstatuses"}。componentstatuses用于检查集群核心组件（如scheduler、controller-manager、etcd等）的健康状态。`

	reqBody := ChatRequest{
		Model: modelName,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
	}

	var resp ChatResponse
	_, err := client.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		SetResult(&resp).
		Post(baseURL)

	if err != nil {
		return nil, err
	}

	raw := resp.Choices[0].Message.Content

	// 处理可能的markdown代码块包裹
	if strings.HasPrefix(raw, "```json") && strings.HasSuffix(raw, "```") {
		// 去除markdown代码块标记
		lines := strings.Split(raw, "\n")
		if len(lines) > 2 {
			raw = strings.Join(lines[1:len(lines)-1], "\n")
		}
	} else if strings.HasPrefix(raw, "```") && strings.HasSuffix(raw, "```") {
		// 去除普通代码块标记
		lines := strings.Split(raw, "\n")
		if len(lines) > 2 {
			raw = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var intent Intent
	if err := json.Unmarshal([]byte(raw), &intent); err != nil {
		return nil, fmt.Errorf("parse JSON failed: %v\nraw: %s", err, raw)
	}
	return &intent, nil
}

// 添加AI自然语言总结函数
func SummarizeWithAI(baseURL, apiKey, modelName, formattedOutput, resource, query string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY not set")
	}

	client := resty.New()
	systemPrompt := `你是一个专业的Kubernetes运维助手。用户会提供Kubernetes资源的表格化输出数据，你需要将其转换为自然、易懂的中文描述。

要求：
1. 用自然语言描述资源的整体状况
2. 突出重要信息（如资源数量、状态、异常情况等）
3. 如果有多个命名空间，按命名空间分组描述
4. 语言要专业但易懂，适合运维人员阅读
5. 如果发现异常状态，要特别提醒
6. 保持简洁，避免冗余信息`

	userPrompt := fmt.Sprintf("用户查询：%s\n\n表格化输出结果：\n%s\n\n请将上述结果整理成自然语言描述。", query, formattedOutput)

	reqBody := ChatRequest{
		Model: modelName,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	var resp ChatResponse
	_, err := client.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		SetResult(&resp).
		Post(baseURL)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return resp.Choices[0].Message.Content, nil
}
