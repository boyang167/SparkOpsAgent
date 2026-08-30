import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesConfigsPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "配置管理",
    description: "查看 ConfigMap 与 Secret。",
    menu: "configs",
  });
}
