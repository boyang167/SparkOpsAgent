export type PodEvent = Record<string, unknown>;

const VOLUME_SOURCE_KEYS = [
  "hostPath",
  "configMap",
  "secret",
  "persistentVolumeClaim",
  "emptyDir",
  "downwardAPI",
  "projected",
] as const;

export function eventTimestamp(event: PodEvent): string | undefined {
  const lastTimestamp = event.lastTimestamp;
  if (typeof lastTimestamp === "string" && lastTimestamp) return lastTimestamp;

  const eventTime = event.eventTime;
  if (typeof eventTime === "string" && eventTime) return eventTime;

  const metadata = event.metadata;
  if (metadata && typeof metadata === "object" && !Array.isArray(metadata)) {
    const creationTimestamp = (metadata as Record<string, unknown>).creationTimestamp;
    if (typeof creationTimestamp === "string" && creationTimestamp) return creationTimestamp;
  }

  return undefined;
}

export function sortPodEvents(events: PodEvent[]): PodEvent[] {
  return [...events].sort((a, b) => {
    const ta = eventTimestamp(a);
    const tb = eventTimestamp(b);
    if (!ta && !tb) return 0;
    if (!ta) return 1;
    if (!tb) return -1;
    return new Date(tb).getTime() - new Date(ta).getTime();
  });
}

export function formatContainerPorts(container: Record<string, unknown>): string {
  const ports = container.ports;
  if (!Array.isArray(ports) || ports.length === 0) return "-";

  const formatted = ports
    .map((port) => {
      if (!port || typeof port !== "object") return null;
      const item = port as Record<string, unknown>;
      const protocol = typeof item.protocol === "string" && item.protocol ? item.protocol : "TCP";
      const containerPort = item.containerPort;
      if (containerPort === undefined || containerPort === null) return null;
      return `${protocol}: ${safeString(containerPort)}`;
    })
    .filter((item): item is string => Boolean(item));

  return formatted.length > 0 ? formatted.join(", ") : "-";
}

export function formatContainerCommand(container: Record<string, unknown>): string {
  const command = stringifyCommandParts(container.command);
  const args = stringifyCommandParts(container.args);
  const parts = [command, args].filter(Boolean);
  return parts.length > 0 ? parts.join(" ") : "-";
}

export function formatEnvironment(container: Record<string, unknown>): string[] {
  const env = container.env;
  if (!Array.isArray(env)) return [];

  return env
    .map((item) => {
      if (!item || typeof item !== "object") return null;
      const entry = item as Record<string, unknown>;
      const name = typeof entry.name === "string" ? entry.name : safeString(entry.name);
      if (!name) return null;

      if (typeof entry.value === "string") return `${name}=${entry.value}`;
      if (entry.value !== undefined && entry.value !== null) return `${name}=${safeString(entry.value)}`;

      const valueFrom = entry.valueFrom;
      if (valueFrom && typeof valueFrom === "object" && !Array.isArray(valueFrom)) {
        return `${name}=${formatValueFrom(valueFrom as Record<string, unknown>)}`;
      }

      return `${name}=`;
    })
    .filter((item): item is string => Boolean(item));
}

export function volumeSourceView(volume: Record<string, unknown>): { type: string; detail: string } {
  for (const key of VOLUME_SOURCE_KEYS) {
    const source = volume[key];
    if (source && typeof source === "object" && !Array.isArray(source)) {
      return { type: key, detail: formatVolumeDetail(source as Record<string, unknown>) };
    }
  }

  const unknownKey = Object.keys(volume).find((key) => key !== "name");
  if (!unknownKey) return { type: "-", detail: "-" };

  return {
    type: unknownKey,
    detail: safeJsonStringify(volume[unknownKey]),
  };
}

function stringifyCommandParts(value: unknown): string {
  if (!Array.isArray(value) || value.length === 0) return "";
  return value.map((part) => safeString(part)).join(" ");
}

function formatValueFrom(valueFrom: Record<string, unknown>): string {
  const fieldRef = valueFrom.fieldRef;
  if (fieldRef && typeof fieldRef === "object" && !Array.isArray(fieldRef)) {
    const fieldPath = (fieldRef as Record<string, unknown>).fieldPath;
    return `<fieldRef ${safeString(fieldPath)}>`;
  }

  const configMapKeyRef = valueFrom.configMapKeyRef;
  if (configMapKeyRef && typeof configMapKeyRef === "object" && !Array.isArray(configMapKeyRef)) {
    const ref = configMapKeyRef as Record<string, unknown>;
    return `<configMapKeyRef ${safeString(ref.name)}/${safeString(ref.key)}>`;
  }

  const secretKeyRef = valueFrom.secretKeyRef;
  if (secretKeyRef && typeof secretKeyRef === "object" && !Array.isArray(secretKeyRef)) {
    const ref = secretKeyRef as Record<string, unknown>;
    return `<secretKeyRef ${safeString(ref.name)}/${safeString(ref.key)}>`;
  }

  return `<valueFrom: ${safeJsonStringify(valueFrom)}>`;
}

function formatVolumeDetail(source: Record<string, unknown>): string {
  const parts = Object.entries(source).map(([key, value]) => `${key}: ${formatVolumeValue(value)}`);
  return parts.length > 0 ? parts.join(", ") : "-";
}

function formatVolumeValue(value: unknown): string {
  if (value === undefined || value === null) return "-";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return safeJsonStringify(value);
}

function safeString(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return safeJsonStringify(value);
}

function safeJsonStringify(value: unknown): string {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}
