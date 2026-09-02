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
