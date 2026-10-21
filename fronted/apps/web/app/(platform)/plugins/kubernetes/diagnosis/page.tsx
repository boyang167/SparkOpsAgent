import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesDiagnosisPage() {
  return renderKubernetesPage({
    mode: "diagnosis",
    title: "应用诊断",
    description: "启动 Arthas 诊断会话并读取后端 SSE 输出。",
  });
}
