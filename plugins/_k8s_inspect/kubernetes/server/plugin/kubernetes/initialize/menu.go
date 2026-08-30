package initialize

import (
	"context"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/plugin-tool/utils"
)

func Menu(ctx context.Context) {
	entities := []model.SysBaseMenu{
		{
			Name:      "Kubernetes",
			Path:      "kubernetes",
			Hidden:    false,
			Component: "plugin/kubernetes/view/index.vue",
			Sort:      1000,
			Meta:      model.Meta{Title: "Kubernetes管理", Icon: "cloudy"},
		},
		{
			Name:      "clusters",
			Path:      "clusters",
			Hidden:    false,
			Component: "plugin/kubernetes/view/clusters/index.vue",
			Sort:      0,
			Meta:      model.Meta{Title: "集群管理", Icon: "cloudy"},
		},
		{
			Name:      "nodes",
			Path:      "nodes",
			Hidden:    false,
			Component: "plugin/kubernetes/view/nodes/index.vue",
			Sort:      1,
			Meta:      model.Meta{Title: "节点管理", Icon: "film"},
		},
		{
			Name:      "nodelabelstaints",
			Path:      "nodelabelstaints",
			Hidden:    true,
			Component: "plugin/kubernetes/view/nodelabelstaints/index.vue",
			Sort:      2,
			Meta:      model.Meta{Title: "标签与污点", Icon: "star"},
		},
		{
			Name:      "workloads",
			Path:      "workloads",
			Hidden:    false,
			Component: "plugin/kubernetes/view/workloads/index.vue",
			Sort:      3,
			Meta:      model.Meta{Title: "工作负载", Icon: "position"},
		},
		{
			Name:      "openkruise",
			Path:      "openkruise",
			Hidden:    false,
			Component: "plugin/kubernetes/view/openkruise/index.vue",
			Sort:      4,
			Meta:      model.Meta{Title: "OpenKruise", Icon: "position"},
		},
		{
			Name:      "kruise-rollouts",
			Path:      "kruise-rollouts",
			Hidden:    false,
			Component: "plugin/kubernetes/view/kruise/index.vue",
			Sort:      5,
			Meta:      model.Meta{Title: "Kruise Rollouts", Icon: "position"},
		},
		{
			Name:      "kubeblocks",
			Path:      "kubeblocks",
			Hidden:    false,
			Component: "plugin/kubernetes/view/kubeblocks/index.vue",
			Sort:      6,
			Meta:      model.Meta{Title: "KubeBlockss", Icon: "position"},
		},
		{
			Name:      "kubevela",
			Path:      "kubevela",
			Hidden:    false,
			Component: "plugin/kubernetes/view/kubevela/index.vue",
			Sort:      7,
			Meta:      model.Meta{Title: "Kubevela", Icon: "position"},
		},
		{
			Name:      "namespaces",
			Path:      "namespaces",
			Hidden:    false,
			Component: "plugin/kubernetes/view/namespaces/index.vue",
			Sort:      8,
			Meta:      model.Meta{Title: "命名空间", Icon: "crop"},
		},
		{
			Name:      "networks",
			Path:      "networks",
			Hidden:    false,
			Component: "plugin/kubernetes/view/networks/index.vue",
			Sort:      9,
			Meta:      model.Meta{Title: "网络管理", Icon: "ship"},
		},
		{
			Name:      "gateway",
			Path:      "gateway",
			Hidden:    false,
			Component: "plugin/kubernetes/view/gateway/index.vue",
			Sort:      10,
			Meta:      model.Meta{Title: "Gateway", Icon: "rank"},
		},
		{
			Name:      "configs",
			Path:      "configs",
			Hidden:    false,
			Component: "plugin/kubernetes/view/configs/index.vue",
			Sort:      11,
			Meta:      model.Meta{Title: "配置管理", Icon: "scale-to-original"},
		},
		{
			Name:      "storages",
			Path:      "storages",
			Hidden:    false,
			Component: "plugin/kubernetes/view/storages/index.vue",
			Sort:      12,
			Meta:      model.Meta{Title: "存储管理", Icon: "coin"},
		},
		{
			Name:      "accessControls",
			Path:      "accessControls",
			Hidden:    false,
			Component: "plugin/kubernetes/view/accessControls/index.vue",
			Sort:      13,
			Meta:      model.Meta{Title: "访问控制", Icon: "key"},
		},
		{
			Name:      "records",
			Path:      "records",
			Hidden:    false,
			Component: "plugin/kubernetes/view/record/index.vue",
			Sort:      14,
			Meta:      model.Meta{Title: "终端审计", Icon: "pie-chart"},
		},
		{
			Name:      "diagnosis",
			Path:      "diagnosis",
			Hidden:    false,
			Component: "plugin/kubernetes/view/diagnosis/index.vue",
			Sort:      15,
			Meta:      model.Meta{Title: "应用诊断", Icon: "tools"},
		},
	}
	utils.RegisterMenus(entities...)
}
