export const CONFIG_TABS = [
  "ConfigMap",
  "Secret",
  "ResourceQuota",
  "LimitRange",
  "HorizontalPodAutoscaler",
  "PodDisruptionBudget",
] as const;

export type ConfigTab = (typeof CONFIG_TABS)[number];

const CONFIG_API_VERSION: Record<ConfigTab, string> = {
  ConfigMap: "v1",
  Secret: "v1",
  ResourceQuota: "v1",
  LimitRange: "v1",
  HorizontalPodAutoscaler: "autoscaling/v2",
  PodDisruptionBudget: "policy/v1",
};

const EDITABLE_SECRET_TYPES = new Set(["Opaque", "kubernetes.io/dockerconfigjson", "kubernetes.io/tls"]);

export function configTabsForUser(canManage: boolean): ConfigTab[] {
  return CONFIG_TABS.filter((tab) => tab !== "Secret" || canManage);
}

export function configTabLabel(tab: ConfigTab): string {
  if (tab === "ConfigMap") return "Config Maps";
  if (tab === "Secret") return "Secrets";
  if (tab === "ResourceQuota") return "Resource Quotas";
  if (tab === "LimitRange") return "Limit Ranges";
  if (tab === "HorizontalPodAutoscaler") return "HPA";
  if (tab === "PodDisruptionBudget") return "Pod Disruption Budget";
  return tab;
}

export function secretEditableType(type?: string): boolean {
  return EDITABLE_SECRET_TYPES.has(type ?? "");
}

export function emptyConfigTemplate(kind: ConfigTab, namespace?: string): Record<string, unknown> {
  const tpl: Record<string, unknown> = {
    apiVersion: CONFIG_API_VERSION[kind],
    kind,
    metadata: namespace ? { namespace } : {},
  };
  if (kind === "ConfigMap") {
    tpl.data = {};
  }
  if (kind === "Secret") {
    tpl.type = "Opaque";
    tpl.data = {};
  }
  return tpl;
}

export function configTableColSpan(tab: ConfigTab): number {
  return tab === "Secret" ? 6 : 5;
}
