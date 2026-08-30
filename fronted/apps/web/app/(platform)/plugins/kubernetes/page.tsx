import { redirect } from "next/navigation";

export default function KubernetesPluginPage() {
  redirect("/plugins/kubernetes/clusters");
}
