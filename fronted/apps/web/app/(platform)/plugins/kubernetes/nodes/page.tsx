import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesNodesPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "节点管理",
    description: "查看节点状态、资源用量，并执行调度、排空与终端操作。",
    menu: "nodes",
  });
}
