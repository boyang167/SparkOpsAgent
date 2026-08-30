import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesNamespacesPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "命名空间",
    description: "查看 Namespace 与 Event。",
    menu: "namespaces",
  });
}
