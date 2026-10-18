export interface KubernetesSectionItem {
  href: string;
  label: string;
}

export interface KubernetesSection {
  href: string;
  label: string;
  items: KubernetesSectionItem[];
}

export const KUBERNETES_SECTIONS: KubernetesSection[] = [
  {
    href: "/plugins/kubernetes/clusters",
    label: "集群管理",
    items: [{ href: "/plugins/kubernetes/clusters", label: "集群管理" }],
  },
  {
    href: "/plugins/kubernetes/workloads",
    label: "工作负载",
    items: [{ href: "/plugins/kubernetes/workloads", label: "工作负载" }],
  },
  {
    href: "/plugins/kubernetes/nodes",
    label: "节点管理",
    items: [{ href: "/plugins/kubernetes/nodes", label: "节点管理" }],
  },
  {
    href: "/plugins/kubernetes/namespaces",
    label: "命名空间",
    items: [{ href: "/plugins/kubernetes/namespaces", label: "命名空间" }],
  },
  {
    href: "/plugins/kubernetes/networks",
    label: "网络与网关",
    items: [
      { href: "/plugins/kubernetes/networks", label: "网络资源" },
      { href: "/plugins/kubernetes/gateway", label: "Gateway" },
    ],
  },
  {
    href: "/plugins/kubernetes/configs",
    label: "配置与存储",
    items: [
      { href: "/plugins/kubernetes/configs", label: "配置" },
      { href: "/plugins/kubernetes/storages", label: "存储" },
    ],
  },
  {
    href: "/plugins/kubernetes/access-controls",
    label: "安全与访问",
    items: [
      { href: "/plugins/kubernetes/access-controls", label: "访问控制" },
      { href: "/plugins/kubernetes/records", label: "终端审计" },
    ],
  },
  {
    href: "/plugins/kubernetes/diagnosis",
    label: "运维工具",
    items: [
      { href: "/plugins/kubernetes/diagnosis", label: "应用诊断" },
      { href: "/plugins/kubernetes/cloudtty", label: "CloudTTY" },
      { href: "/plugins/kubernetes/backup", label: "备份" },
      { href: "/plugins/kubernetes/chat", label: "AI Chat" },
      { href: "/plugins/kubernetes/openkruise", label: "OpenKruise" },
      { href: "/plugins/kubernetes/kruise-rollouts", label: "Kruise Rollouts" },
      { href: "/plugins/kubernetes/kubeblocks", label: "KubeBlocks" },
      { href: "/plugins/kubernetes/kubevela", label: "KubeVela" },
      { href: "/plugins/kubernetes/velaux", label: "Velaux" },
    ],
  },
];

export function kubernetesPathMatches(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function sectionForPath(pathname: string): KubernetesSection | undefined {
  return KUBERNETES_SECTIONS.find((section) =>
    section.items.some((item) => kubernetesPathMatches(pathname, item.href)),
  );
}

export function visibleSectionForPath(
  pathname: string,
  visiblePaths: ReadonlySet<string>,
): KubernetesSection | undefined {
  const section = sectionForPath(pathname);
  if (!section) return undefined;
  const items = section.items.filter((item) => visiblePaths.has(item.href));
  return items.length > 0 ? { ...section, items } : undefined;
}
