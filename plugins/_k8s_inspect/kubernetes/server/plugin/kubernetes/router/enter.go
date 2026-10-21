package router

type RouterGroup struct {
	ClusterRouter
	ProxyApiRouter
	MetricRouter
	WsApiRouter
	PodsRouter
	RecordsRouter
	VelauxApiRouter
	CloudTTYApiRouter
	NodeTTYApiRouter
	DiagnosisRouter
	ChatApiRouter
}

var RouterGroupApp = new(RouterGroup)
