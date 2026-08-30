package utils

import (
	"encoding/json"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
)

func KubeconfigJson(cluster model.Cluster, username, privateKey, publicCert string) (yaml string, err error) {
	// 创建Kubeconfig结构
	kubeconfig := model.Kubeconfig{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []model.ClusterEntry{
			{
				Name:    cluster.Name,
				Cluster: model.KubeCluster{CertificateAuthorityData: cluster.KubeCaCrt, Server: cluster.ApiAddress},
			},
		},
		Contexts: []model.ContextEntry{
			{
				Name: cluster.Name,
				Context: model.Context{
					Cluster: cluster.Name,
					User:    username,
				},
			},
		},
		CurrentContext: cluster.Name,
		Preferences:    struct{}{},
		Users: []model.UserEntry{
			{
				Name: username,
				User: model.KubeUser{
					ClientCertificateData: publicCert,
					ClientKeyData:         privateKey,
				},
			},
		},
	}

	// 将Kubeconfig结构转换为Json格式的字节
	jsonData, err := json.Marshal(&kubeconfig)
	if err != nil {
		return "", err
	}

	return string(jsonData), err
}
