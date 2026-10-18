export const ENVIRONMENT_SYNC_RESOURCE_KINDS = [
  { id: "Deployment", label: "Deployment", hint: "工作负载" },
  { id: "Service", label: "Service", hint: "服务访问" },
  { id: "ConfigMap", label: "ConfigMap", hint: "配置项" },
  { id: "Ingress", label: "Ingress", hint: "入口路由" },
] as const;

export type EnvironmentSyncResourceKind = (typeof ENVIRONMENT_SYNC_RESOURCE_KINDS)[number]["id"];

export const DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS: EnvironmentSyncResourceKind[] =
  ENVIRONMENT_SYNC_RESOURCE_KINDS.map((item) => item.id);

const allowedKindSet = new Set<string>(DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS);

export function normalizeSyncResourceKinds(
  kinds: string[] | undefined,
  syncYaml: boolean,
  firstSync: boolean,
): EnvironmentSyncResourceKind[] {
  if (!syncYaml) return [];
  if (firstSync) return [...DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS];
  const selected = (kinds ?? [])
    .map((item) => item.trim())
    .filter((item): item is EnvironmentSyncResourceKind => allowedKindSet.has(item));
  return selected.length > 0 ? selected : [...DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS];
}

export function formatSyncResourceKinds(kinds: string[]): string {
  if (kinds.length === 0) return "未选择";
  const labels = kinds.map((kind) => ENVIRONMENT_SYNC_RESOURCE_KINDS.find((item) => item.id === kind)?.label ?? kind);
  return labels.join("、");
}

export function toggleSyncResourceKind(
  kinds: string[],
  kind: EnvironmentSyncResourceKind,
  selected: boolean,
): EnvironmentSyncResourceKind[] {
  const set = new Set(kinds.filter((item) => allowedKindSet.has(item)));
  if (selected) set.add(kind);
  else set.delete(kind);
  return DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS.filter((item) => set.has(item));
}
