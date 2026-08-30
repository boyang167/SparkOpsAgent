import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesClustersPage() {
  return renderKubernetesPage({
    mode: "clusters",
    title: "集群管理",
    description: "查看当前工作区授权集群，并维护 Prometheus、Velaux、LLM 设置。",
  });
}
