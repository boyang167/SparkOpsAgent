import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesRecordsPage() {
  return renderKubernetesPage({
    mode: "records",
    title: "终端审计",
    description: "查看、回放和删除 Kubernetes 终端审计记录。",
  });
}
