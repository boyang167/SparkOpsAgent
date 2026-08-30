import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesWorkloadsPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "工作负载",
    description: "查看 Pod、Deployment、DaemonSet、StatefulSet、ReplicaSet、Job、CronJob。",
    menu: "workloads",
  });
}
