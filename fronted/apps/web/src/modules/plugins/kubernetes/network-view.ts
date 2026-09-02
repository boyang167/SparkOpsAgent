export const NETWORK_TABS = ["Service", "Ingress", "Endpoints", "NetworkPolicy"] as const;

export type NetworkTab = (typeof NETWORK_TABS)[number];

type ServicePort = {
  port?: number;
  nodePort?: number;
  protocol?: string;
};

type LoadBalancerIngress = {
  ip?: string;
  hostname?: string;
};

type IngressPathBackend = {
  service?: { name?: string; port?: { number?: number } };
  serviceName?: string;
  servicePort?: number | string;
};

type IngressRule = {
  host?: string;
  http?: { paths?: Array<{ path?: string; backend?: IngressPathBackend }> };
};

type EndpointSubset = {
  addresses?: Array<{ ip?: string }>;
  ports?: Array<{ port?: number }>;
};

const NETWORK_API_VERSION: Record<NetworkTab, string> = {
  Service: "v1",
  Ingress: "networking.k8s.io/v1",
  Endpoints: "v1",
  NetworkPolicy: "networking.k8s.io/v1",
};

export function networkTabLabel(tab: NetworkTab): string {
  if (tab === "Service") return "Services";
  if (tab === "NetworkPolicy") return "Network Policies";
  return tab;
}

export function emptyNetworkTemplate(kind: NetworkTab, namespace?: string): Record<string, unknown> {
  const tpl: Record<string, unknown> = {
    apiVersion: NETWORK_API_VERSION[kind],
    kind,
    metadata: namespace ? { namespace } : {},
  };
  if (kind === "Service") {
    tpl.spec = { type: "ClusterIP", ports: [], selector: {} };
  }
  return tpl;
}

export function formatExternalIPs(ips?: string[]): string {
  if (!ips?.length) return "-";
  return ips.join(", ");
}

export function formatServiceInternalEndpoints(name: string, ports?: ServicePort[]): string[] {
  if (!ports?.length) return [];
  const lines: string[] = [];
  for (const item of ports) {
    if (item.port) lines.push(`${name}: ${item.port} ${item.protocol ?? "TCP"}`);
    if (item.nodePort) lines.push(`${name}: ${item.nodePort} ${item.protocol ?? "TCP"}`);
  }
  return lines;
}

export function formatServiceExternalEndpoints(
  ports?: ServicePort[],
  ingress?: LoadBalancerIngress[],
): string[] {
  if (!ingress?.length || !ports?.length) return [];
  const lines: string[] = [];
  for (const entry of ingress) {
    const host = entry.ip ?? entry.hostname ?? "";
    if (!host) continue;
    for (const item of ports) {
      if (item.port) lines.push(`${host}: ${item.port}`);
    }
  }
  return lines;
}

export function formatSelector(selector?: Record<string, string>): string[] {
  if (!selector) return [];
  return Object.entries(selector).map(([key, value]) => `${key}=${value}`);
}

export function formatIngressRules(rules?: IngressRule[]): string[] {
  if (!rules?.length) return [];
  const lines: string[] = [];
  for (const rule of rules) {
    const host = rule.host ?? "";
    for (const path of rule.http?.paths ?? []) {
      const backend = path.backend;
      const serviceName = backend?.service?.name ?? backend?.serviceName ?? "";
      const servicePort = backend?.service?.port?.number ?? backend?.servicePort ?? "";
      lines.push(`${host}${path.path ?? ""} -> ${serviceName}:${servicePort}`);
    }
  }
  return lines;
}

export function formatLoadBalancerIngress(ingress?: LoadBalancerIngress[]): string[] {
  if (!ingress?.length) return [];
  return ingress.map((item) => item.ip ?? item.hostname ?? "").filter(Boolean);
}

export function formatEndpointSubsets(subsets?: EndpointSubset[]): string[] {
  if (!subsets?.length) return [];
  const lines: string[] = [];
  for (const subset of subsets) {
    for (const address of subset.addresses ?? []) {
      for (const port of subset.ports ?? []) {
        if (address.ip && port.port) lines.push(`${address.ip}:${port.port}`);
      }
    }
  }
  return lines;
}

export function networkTableColSpan(tab: NetworkTab): number {
  if (tab === "Service") return 11;
  if (tab === "Ingress") return 7;
  if (tab === "Endpoints") return 6;
  return 5;
}
