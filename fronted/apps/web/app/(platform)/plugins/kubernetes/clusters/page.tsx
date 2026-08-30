import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesClustersPage() {
  return renderKubernetesPage({
    mode: "clusters",
    title: "集群管理",
    description: "在当前工作区登记 Kubernetes 集群，并维护 Prometheus、Velaux、LLM 设置。",
  });
}
