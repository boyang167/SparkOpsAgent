export const STORAGE_TABS = ["PersistentVolumeClaim", "PersistentVolume", "StorageClass"] as const;

export type StorageTab = (typeof STORAGE_TABS)[number];

const STORAGE_API_VERSION: Record<StorageTab, string> = {
  PersistentVolumeClaim: "v1",
  PersistentVolume: "v1",
  StorageClass: "storage.k8s.io/v1",
};

export function storageTabLabel(tab: StorageTab): string {
  if (tab === "PersistentVolumeClaim") return "Persistent VolumeClaims";
  if (tab === "PersistentVolume") return "Persistent Volumes";
  if (tab === "StorageClass") return "Storage Class";
  return tab;
}

export function storagePhaseBadge(phase?: string): { text: string; tone: "ok" | "bad" | "muted" } {
  if (phase === "Bound" || phase === "Available") return { text: phase === "Available" ? "Available" : "Bound", tone: "ok" };
  if (!phase) return { text: "-", tone: "muted" };
  return { text: phase, tone: "bad" };
}

export function emptyStorageTemplate(kind: StorageTab, namespace?: string): Record<string, unknown> {
  const tpl: Record<string, unknown> = {
    apiVersion: STORAGE_API_VERSION[kind],
    kind,
    metadata: namespace && kind === "PersistentVolumeClaim" ? { namespace } : {},
  };
  if (kind === "PersistentVolumeClaim") {
    tpl.spec = {
      accessModes: ["ReadWriteOnce"],
      resources: { requests: { storage: "1Gi" } },
    };
  }
  if (kind === "PersistentVolume") {
    tpl.spec = {
      capacity: { storage: "1Gi" },
      accessModes: ["ReadWriteOnce"],
      persistentVolumeReclaimPolicy: "Retain",
    };
  }
  if (kind === "StorageClass") {
    tpl.provisioner = "kubernetes.io/no-provisioner";
    tpl.reclaimPolicy = "Delete";
  }
  return tpl;
}

export function storageTableColSpan(tab: StorageTab): number {
  if (tab === "PersistentVolumeClaim") return 10;
  if (tab === "PersistentVolume") return 10;
  return 6;
}

export function readNestedString(obj: Record<string, unknown>, path: string[]): string {
  let current: unknown = obj;
  for (const key of path) {
    if (!current || typeof current !== "object" || Array.isArray(current)) return "-";
    current = (current as Record<string, unknown>)[key];
  }
  return typeof current === "string" && current.trim() ? current : "-";
}

export function formatAccessModes(modes?: string[]): string {
  if (!modes?.length) return "-";
  return modes.join(", ");
}
