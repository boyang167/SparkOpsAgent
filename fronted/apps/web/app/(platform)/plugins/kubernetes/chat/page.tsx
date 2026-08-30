import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesChatPage() {
  return renderKubernetesPage({
    mode: "chat",
    title: "AI Chat",
    description: "通过后端 Kubernetes Chat API 发起流式问答。",
  });
}
