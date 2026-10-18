export type NamespaceObject = {
  metadata?: {
    name?: string;
    labels?: Record<string, string>;
    creationTimestamp?: string;
  };
  status?: {
    phase?: string;
  };
};

export function namespacePhaseBadge(phase?: string): { text: string; tone: "ok" | "bad" | "muted" } {
  if (phase === "Active") return { text: "Active", tone: "ok" };
  if (!phase) return { text: "-", tone: "muted" };
  return { text: phase, tone: "bad" };
}

export const NAMESPACE_LABEL_PREVIEW = 2;

/** Labels shown in the table; metadata.name duplicates the name column. */
export function namespaceDisplayLabels(labels?: Record<string, string>): Array<{ key: string; value: string }> {
  return Object.entries(labels ?? {})
    .filter(([key]) => key !== "kubernetes.io/metadata.name")
    .map(([key, value]) => ({ key, value }));
}

export function emptyNamespaceTemplate(): Record<string, unknown> {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name: "",
      labels: {},
    },
  };
}

export type NamespaceLabelRow = { key: string; value: string };

export type NamespaceCreateDraft = {
  name: string;
  labels: NamespaceLabelRow[];
};

export type NamespaceCreateDraftErrors = {
  name?: string;
  labels?: string;
};

const DNS_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

export function emptyNamespaceCreateDraft(): NamespaceCreateDraft {
  return { name: "", labels: [] };
}

export function validateNamespaceName(name: string): string | null {
  const trimmed = name.trim();
  if (!trimmed) return "请填写命名空间名称";
  if (trimmed.length > 63) return "名称不能超过 63 个字符";
  if (!DNS_LABEL.test(trimmed)) {
    return "名称只能包含小写字母、数字和连字符，且须以字母或数字开头和结尾";
  }
  return null;
}

export function labelsFromRows(rows: NamespaceLabelRow[]): { labels?: Record<string, string>; error?: string } {
  const labels: Record<string, string> = {};
  for (const row of rows) {
    const key = row.key.trim();
    const value = row.value.trim();
    if (!key && !value) continue;
    if (!key) return { error: "标签键不能为空" };
    if (key in labels) return { error: `标签键「${key}」重复` };
    labels[key] = value;
  }
  return { labels };
}

export function validateNamespaceCreateDraft(draft: NamespaceCreateDraft): NamespaceCreateDraftErrors {
  const errors: NamespaceCreateDraftErrors = {};
  const nameError = validateNamespaceName(draft.name);
  if (nameError) errors.name = nameError;
  const { error: labelsError } = labelsFromRows(draft.labels);
  if (labelsError) errors.labels = labelsError;
  return errors;
}

export function buildNamespaceFromDraft(draft: NamespaceCreateDraft): Record<string, unknown> {
  const name = draft.name.trim();
  const { labels = {} } = labelsFromRows(draft.labels);
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name,
      ...(Object.keys(labels).length > 0 ? { labels } : {}),
    },
  };
}
