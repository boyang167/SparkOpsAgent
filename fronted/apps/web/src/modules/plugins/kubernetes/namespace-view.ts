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

export type NamespaceLabelRow = {
  key: string;
  value: string;
};

export type NamespaceCreateDraft = {
  name: string;
  labels: NamespaceLabelRow[];
};

export type NamespaceCreateDraftErrors = {
  name?: string;
  labels?: string;
};

export function emptyNamespaceCreateDraft(): NamespaceCreateDraft {
  return { name: "", labels: [] };
}

function isNamespaceName(value: string): boolean {
  const trimmed = value.trim();
  return trimmed.length > 0 && trimmed.length <= 63 && /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(trimmed);
}

function isLabelKey(value: string): boolean {
  const trimmed = value.trim();
  const slash = trimmed.lastIndexOf("/");
  const name = slash >= 0 ? trimmed.slice(slash + 1) : trimmed;
  const prefix = slash >= 0 ? trimmed.slice(0, slash) : "";
  const nameOk = name.length > 0 && name.length <= 63 && /^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$/.test(name);
  if (!prefix) return nameOk;
  return (
    nameOk &&
    prefix.length <= 253 &&
    prefix.split(".").every((part) => part.length > 0 && part.length <= 63 && /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(part))
  );
}

export function validateNamespaceCreateDraft(draft: NamespaceCreateDraft): NamespaceCreateDraftErrors {
  const errors: NamespaceCreateDraftErrors = {};
  if (!isNamespaceName(draft.name)) {
    errors.name = "名称必须是小写字母、数字与连字符，且以字母或数字开头和结尾，最长 63 字符";
  }
  const filled = draft.labels.filter((row) => row.key.trim() || row.value.trim());
  if (filled.some((row) => !isLabelKey(row.key))) {
    errors.labels = "标签键必须符合 Kubernetes 标签格式";
  }
  return errors;
}

export function buildNamespaceFromDraft(draft: NamespaceCreateDraft): Record<string, unknown> {
  const labels: Record<string, string> = {};
  for (const row of draft.labels) {
    const key = row.key.trim();
    if (!key) continue;
    labels[key] = row.value;
  }
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name: draft.name.trim(),
      labels,
    },
  };
}
