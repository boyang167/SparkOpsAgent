package api

import "github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/service"

type ApiGroup struct {
	ClustersApi
	ProxyApi
	MetricsApi
	WsApi
	PodsApi
	RecordsApi
	VelauxApi
	CloudTTYApi
	NodeTTYApi
	DiagnosisApi
	ChatApi
}

var ApiGroupApp = new(ApiGroup)

var (
	clusterService     = service.ServiceGroupApp.ClusterService
	metricService      = service.ServiceGroupApp.MetricsService
	proxyService       = service.ServiceGroupApp.ProxyService
	podsService        = service.ServiceGroupApp.PodsService
	recordsService     = service.ServiceGroupApp.RecordsService
	velauxService      = service.ServiceGroupApp.VelauxService
	cloudttyService    = service.ServiceGroupApp.CloudTTYService
	nodettyService     = service.ServiceGroupApp.NodeTTYService
	diagnosisService   = service.ServiceGroupApp.DiagnosisService
	chatServiceService = service.ServiceGroupApp.ChatService
)
