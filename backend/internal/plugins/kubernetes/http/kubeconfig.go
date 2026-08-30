package http

import (
	stdhttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type kubeconfigView struct {
	Kubeconfig string `json:"kubeconfig"`
}

func (h clusterHandler) getKubeconfig(c *gin.Context) {
	if !h.writeReady(c) {
		return
	}
	wsID, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", stdhttp.StatusBadRequest)
		return
	}
	_, _, restCfg, meta, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	name := ""
	if meta != nil {
		name = meta.Name
	}
	yaml, err := kubeconfigYAMLFromREST(restCfg, name)
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_KUBECONFIG_FAILED", "failed to build kubeconfig", stdhttp.StatusInternalServerError)
		return
	}
	httpx.OK(c, kubeconfigView{Kubeconfig: yaml})
}

func kubeconfigYAMLFromREST(cfg *rest.Config, clusterName string) (string, error) {
	if cfg == nil {
		return "", kernel.NewDomainError("PLUGIN_CLUSTER_KUBECONFIG_FAILED", "failed to build kubeconfig")
	}
	name := strings.TrimSpace(clusterName)
	if name == "" {
		name = "cluster"
	}
	user := name + "-user"
	apiCfg := clientcmdapi.NewConfig()
	apiCfg.Clusters[name] = &clientcmdapi.Cluster{
		Server:                   strings.TrimSpace(cfg.Host),
		CertificateAuthorityData: append([]byte(nil), cfg.CAData...),
		InsecureSkipTLSVerify:    cfg.Insecure,
	}
	apiCfg.AuthInfos[user] = &clientcmdapi.AuthInfo{
		Token:                 cfg.BearerToken,
		ClientCertificateData: append([]byte(nil), cfg.CertData...),
		ClientKeyData:         append([]byte(nil), cfg.KeyData...),
	}
	apiCfg.Contexts[name] = &clientcmdapi.Context{
		Cluster:  name,
		AuthInfo: user,
	}
	apiCfg.CurrentContext = name
	raw, err := clientcmd.Write(*apiCfg)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
