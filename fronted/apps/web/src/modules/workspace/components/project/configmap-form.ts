import { parseYamlEditorDocument, stringifyYamlEditorDocument } from "@/modules/plugins/kubernetes/yaml-document";

export type ConfigMapRow = {
  id: string;
  key: string;
  value: string;
};

const KEY_PATTERN = /^[-._a-zA-Z0-9]+$/;
const NAME_PATTERN = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

export function newConfigMapRow(partial?: Partial<ConfigMapRow>): ConfigMapRow {
  return {
    id: partial?.id ?? `row-${Math.random().toString(36).slice(2, 10)}`,
    key: partial?.key ?? "",
    value: partial?.value ?? "",
  };
}

export function sanitizeConfigMapKey(raw: string): string {
  const base = raw.trim().replace(/\\/g, "/").split("/").pop() ?? "";
  const cleaned = base.replace(/[^A-Za-z0-9._-]/g, "-").replace(/-+/g, "-");
  return cleaned.replace(/^[-.]+|[-.]+$/g, "") || "file";
}

export function isValidConfigMapName(name: string): boolean {
  return NAME_PATTERN.test(name) && name.length <= 253;
}

export function isValidConfigMapKey(key: string): boolean {
  return KEY_PATTERN.test(key) && key.length <= 253;
}

export function rowsToData(rows: ConfigMapRow[]): { data: Record<string, string>; error: string | null } {
  const data: Record<string, string> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key && !row.value.trim()) continue;
    if (!key) {
      return { data: {}, error: "存在未填写 Key 的配置行" };
    }
    if (!isValidConfigMapKey(key)) {
      return { data: {}, error: `无效的 Key：${key}（仅支持字母、数字、. _ -）` };
    }
    if (key in data) {
      return { data: {}, error: `Key 重复：${key}` };
    }
    data[key] = row.value;
  }
  return { data, error: null };
}

export function dataToRows(data: Record<string, string> | undefined): ConfigMapRow[] {
  const entries = Object.entries(data ?? {});
  if (entries.length === 0) return [newConfigMapRow()];
  return entries.map(([key, value]) => newConfigMapRow({ key, value }));
}

function stringifyLeaf(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value, null, 2);
}

export function serializeConfigMapYaml(
  name: string,
  namespace: string,
  data: Record<string, string>,
): string {
  return stringifyYamlEditorDocument({
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name,
      namespace,
    },
    data,
  });
}

export function parseConfigMapYaml(text: string): {
  name?: string;
  namespace?: string;
  data: Record<string, string>;
} {
  const doc = parseYamlEditorDocument(text);
  const dataSource =
    doc.data && typeof doc.data === "object" && !Array.isArray(doc.data)
      ? (doc.data as Record<string, unknown>)
      : doc;
  const data: Record<string, string> = {};
  for (const [key, value] of Object.entries(dataSource)) {
    if (["apiVersion", "kind", "metadata"].includes(key) && dataSource === doc) continue;
    if (!isValidConfigMapKey(key)) {
      throw new Error(`无效的 Key：${key}`);
    }
    data[key] = stringifyLeaf(value);
  }
  const metadata =
    doc.metadata && typeof doc.metadata === "object" && !Array.isArray(doc.metadata)
      ? (doc.metadata as Record<string, unknown>)
      : {};
  const name = typeof metadata.name === "string" ? metadata.name : undefined;
  const namespace = typeof metadata.namespace === "string" ? metadata.namespace : undefined;
  return { name, namespace, data };
}

export async function readTextFile(file: File, maxBytes = 1024 * 1024): Promise<string> {
  if (file.size > maxBytes) {
    throw new Error(`文件 ${file.name} 超过 ${Math.round(maxBytes / 1024)}KB 限制`);
  }
  return file.text();
}
