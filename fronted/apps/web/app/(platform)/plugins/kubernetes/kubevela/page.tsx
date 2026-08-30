import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesKubeVelaPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "KubeVela",
    description: "查看 KubeVela Application 资源。",
    menu: "kubevela",
  });
}
