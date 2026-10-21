import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesOpenKruisePage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "OpenKruise",
    description: "查看 CloneSet、AdvancedCronJob、SidecarSet、BroadcastJob。",
    menu: "openkruise",
  });
}
