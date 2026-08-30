import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesNodesPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "节点管理",
    description: "通过 Kubernetes API 查看 Node 资源。",
    menu: "nodes",
  });
}
