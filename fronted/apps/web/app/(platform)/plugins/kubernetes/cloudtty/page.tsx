import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesCloudTTYPage() {
  return renderKubernetesPage({
    mode: "cloudtty",
    title: "CloudTTY",
    description: "创建 CloudTTY 会话并展示后端返回的安全入口信息。",
  });
}
