import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesNetworksPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "网络管理",
    description: "查看 Service、Endpoints、Ingress、NetworkPolicy。",
    menu: "networks",
  });
}
