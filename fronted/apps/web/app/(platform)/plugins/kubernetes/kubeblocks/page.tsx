import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesKubeBlocksPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "KubeBlocks",
    description: "查看 KubeBlocks Cluster 与 ComponentDefinition。",
    menu: "kubeblocks",
  });
}
