import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesCreatePage() {
  return renderKubernetesPage({
    mode: "create",
    title: "创建资源",
    description: "通过向导构造 apps/v1 Deployment JSON 并提交到 Kubernetes API。",
  });
}
