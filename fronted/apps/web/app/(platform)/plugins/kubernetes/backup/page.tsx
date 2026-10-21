import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesBackupPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "Velero 备份",
    description: "查看 Backup、Restore、Schedule。",
    menu: "backup",
  });
}
