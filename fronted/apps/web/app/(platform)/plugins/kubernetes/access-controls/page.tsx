import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesAccessControlsPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "访问控制",
    description: "查看 ServiceAccount、Role、RoleBinding、ClusterRole、ClusterRoleBinding 与旧版 PSP。",
    menu: "access-controls",
  });
}
