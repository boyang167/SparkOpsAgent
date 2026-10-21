import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesStoragesPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "存储管理",
    description: "查看 PersistentVolume 与 PersistentVolumeClaim。",
    menu: "storages",
  });
}
