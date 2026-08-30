import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesVelauxPage() {
  return renderKubernetesPage({
    mode: "velaux",
    title: "Velaux",
    description: "通过 SparkOps BFF 同源代理访问 Velaux。",
  });
}
