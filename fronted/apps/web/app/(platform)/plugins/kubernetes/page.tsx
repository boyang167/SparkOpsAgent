import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesPluginPage() {
  return renderKubernetesPage({
    mode: "overview",
    title: "Kubernetes 插件",
    description: "统一入口覆盖资源目录、集群设置、诊断、终端审计和生态入口。",
  });
}
