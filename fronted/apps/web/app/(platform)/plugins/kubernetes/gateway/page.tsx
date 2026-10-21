import { renderKubernetesPage } from "@/modules/plugins/kubernetes/page-server";

export default function KubernetesGatewayPage() {
  return renderKubernetesPage({
    mode: "resource",
    title: "Gateway API",
    description: "查看 GatewayClass、Gateway、HTTPRoute、GRPCRoute、TCPRoute、TLSRoute、UDPRoute。",
    menu: "gateway",
  });
}
