package service

type ServiceGroup struct {
	ClusterService
	ProxyService
	MetricsService
	PodsService
	RecordsService
	VelauxService
	CloudTTYService
	NodeTTYService
	DiagnosisService
	ChatService
}

var ServiceGroupApp = new(ServiceGroup)
