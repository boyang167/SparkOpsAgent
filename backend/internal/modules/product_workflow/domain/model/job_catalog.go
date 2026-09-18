package model

type JobCatalogItem struct {
	Type        string
	Category    string
	Title       string
	Description string
	Executable  bool
}

func JobCatalog() []JobCatalogItem {
	return []JobCatalogItem{
		{Type: JobTypeSparkopsBuild, Category: "build", Title: "SparkOps 构建", Description: "触发关联项目构建", Executable: true},
		{Type: JobTypeSparkopsDeploy, Category: "deploy", Title: "SparkOps 部署", Description: "部署到产品环境", Executable: true},
		{Type: JobTypeSparkopsApproval, Category: "flow", Title: "人工审批", Description: "人工审批门禁", Executable: true},
		{Type: JobTypeFreestyle, Category: "tools", Title: "自由脚本", Description: "自定义脚本步骤", Executable: true},
		{Type: JobTypeNotify, Category: "tools", Title: "通知", Description: "发送通知消息", Executable: true},
	}
}

func IsKnownJobType(jobType string) bool {
	for _, item := range JobCatalog() {
		if item.Type == jobType && item.Executable {
			return true
		}
	}
	return false
}
