import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesCloudTTYPage() {
  return renderKubernetesPage({
    mode: "cloudtty",
    title: "CloudShell",
    description: "在浏览器中打开容器 Web 终端。",
  });
}
