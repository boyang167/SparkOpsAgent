export type DockerRegistryEntry = {
  registry: string;
  username: string;
  password: string;
};

export type KeyValueEntry = {
  key: string;
  value: string;
};

export type SecretFormState = {
  metadata: {
    name: string;
    namespace: string;
    labels: KeyValueEntry[];
    annotations: KeyValueEntry[];
  };
  type: string;
  opaqueData: KeyValueEntry[];
  dockerEntries: DockerRegistryEntry[];
  tlsCert: string;
  tlsKey: string;
};

export type SecretObject = {
  apiVersion?: string;
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  type?: string;
  data?: Record<string, string>;
};

export function secretTypeLabel(type?: string): string {
  if (type === "kubernetes.io/dockerconfigjson") return "私有镜像仓库登录密钥";
  if (type === "kubernetes.io/tls") return "TLS证书";
  return type ?? "-";
}

export function decodeBase64(value: string): string {
  try {
    return atob(value);
  } catch {
    return value;
  }
}

export function encodeBase64(value: string): string {
  return btoa(value);
}

export function mapToEntries(map?: Record<string, string>): KeyValueEntry[] {
  const entries = Object.entries(map ?? {}).map(([key, value]) => ({ key, value }));
  return entries.length > 0 ? entries : [{ key: "", value: "" }];
}

export function entriesToMap(entries: KeyValueEntry[]): Record<string, string> {
  const map: Record<string, string> = {};
  for (const entry of entries) {
    const key = entry.key.trim();
    if (!key) continue;
    map[key] = entry.value;
  }
  return map;
}

export function decodeOpaqueEntries(data?: Record<string, string>): KeyValueEntry[] {
  const entries = Object.entries(data ?? {}).map(([key, value]) => ({
    key,
    value: decodeBase64(value),
  }));
  return entries.length > 0 ? entries : [{ key: "", value: "" }];
}

export function encodeOpaqueData(entries: KeyValueEntry[]): Record<string, string> {
  const data: Record<string, string> = {};
  for (const entry of entries) {
    const key = entry.key.trim();
    if (!key) continue;
    data[key] = encodeBase64(entry.value);
  }
  return data;
}

export function decodeDockerRegistryEntries(data?: Record<string, string>): DockerRegistryEntry[] {
  const raw = data?.[".dockerconfigjson"];
  if (!raw) return [{ registry: "", username: "", password: "" }];
  try {
    const parsed = JSON.parse(decodeBase64(raw)) as {
      auths?: Record<string, { username?: string; password?: string }>;
    };
    const entries = Object.entries(parsed.auths ?? {}).map(([registry, auth]) => ({
      registry,
      username: auth.username ?? "",
      password: auth.password ?? "",
    }));
    return entries.length > 0 ? entries : [{ registry: "", username: "", password: "" }];
  } catch {
    return [{ registry: "", username: "", password: "" }];
  }
}

export function encodeDockerRegistryData(entries: DockerRegistryEntry[]): Record<string, string> {
  const auths: Record<string, { username: string; password: string; auth: string }> = {};
  for (const entry of entries) {
    const registry = entry.registry.trim();
    if (!registry) continue;
    auths[registry] = {
      username: entry.username,
      password: entry.password,
      auth: encodeBase64(`${entry.username}:${entry.password}`),
    };
  }
  return { ".dockerconfigjson": encodeBase64(JSON.stringify({ auths })) };
}

export function decodeTLSSecret(data?: Record<string, string>): { cert: string; key: string } {
  return {
    cert: decodeBase64(data?.["tls.crt"] ?? ""),
    key: decodeBase64(data?.["tls.key"] ?? ""),
  };
}

export function encodeTLSSecret(cert: string, key: string): Record<string, string> {
  return {
    "tls.crt": encodeBase64(cert),
    "tls.key": encodeBase64(key),
  };
}

export function secretToFormState(secret: SecretObject): SecretFormState {
  const type = secret.type ?? "Opaque";
  return {
    metadata: {
      name: secret.metadata?.name ?? "",
      namespace: secret.metadata?.namespace ?? "",
      labels: mapToEntries(secret.metadata?.labels),
      annotations: mapToEntries(secret.metadata?.annotations),
    },
    type,
    opaqueData: decodeOpaqueEntries(secret.data),
    dockerEntries: decodeDockerRegistryEntries(secret.data),
    tlsCert: decodeTLSSecret(secret.data).cert,
    tlsKey: decodeTLSSecret(secret.data).key,
  };
}

export function formStateToSecret(original: SecretObject, form: SecretFormState): SecretObject {
  const metadata = {
    ...(original.metadata ?? {}),
    name: original.metadata?.name ?? form.metadata.name.trim(),
    namespace: original.metadata?.namespace ?? form.metadata.namespace.trim(),
    labels: entriesToMap(form.metadata.labels),
    annotations: entriesToMap(form.metadata.annotations),
  };

  let data: Record<string, string> = {};
  if (form.type === "Opaque") {
    data = encodeOpaqueData(form.opaqueData);
  } else if (form.type === "kubernetes.io/dockerconfigjson") {
    data = encodeDockerRegistryData(form.dockerEntries);
  } else if (form.type === "kubernetes.io/tls") {
    data = encodeTLSSecret(form.tlsCert, form.tlsKey);
  }

  return {
    ...original,
    apiVersion: original.apiVersion ?? "v1",
    kind: original.kind ?? "Secret",
    metadata,
    type: form.type,
    data,
  };
}
