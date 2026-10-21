import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesKruiseRolloutsPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "Kruise Rollouts",
    description: "查看 Rollout、BatchRelease、TrafficRouting。",
    menu: "kruise-rollouts",
  });
}
