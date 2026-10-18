const SYSTEM_META_PREFIXES = [
  "kubernetes.io/",
  "k8s.io/",
  "node-role.kubernetes.io/",
  "beta.kubernetes.io/",
  "failure-domain.beta.kubernetes.io/",
  "topology.kubernetes.io/",
  "projectcalico.org/",
  "kubeadm.alpha.kubernetes.io/",
  "node.kubernetes.io/",
  "storageprofile.kubernetes.io/",
];

export function isSystemMetaKey(key: string): boolean {
  const lower = key.toLowerCase();
  return SYSTEM_META_PREFIXES.some((prefix) => lower.startsWith(prefix));
}

export function splitMetaEntries(map?: Record<string, string>) {
  const user: Array<[string, string]> = [];
  const system: Array<[string, string]> = [];
  for (const [key, value] of Object.entries(map ?? {})) {
    if (isSystemMetaKey(key)) system.push([key, value]);
    else user.push([key, value]);
  }
  return { user, system };
}

export function formatMetaKey(key: string): string {
  if (key.startsWith("node-role.kubernetes.io/")) {
    return key.slice("node-role.kubernetes.io/".length);
  }
  const slash = key.lastIndexOf("/");
  if (slash >= 0 && slash < key.length - 1) return key.slice(slash + 1);
  return key;
}

export function formatMetaValue(value: string): string {
  if (value === "") return '""';
  return value;
}
