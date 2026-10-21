import { parseDocument, stringify } from "yaml";

const DANGEROUS_KEYS = new Set(["__proto__", "constructor", "prototype"]);

export function parseYamlEditorDocument(text: string): Record<string, unknown> {
  const doc = parseDocument(text, { prettyErrors: false });
  if (doc.errors.length > 0) {
    throw new Error(doc.errors[0]?.message || "YAML/JSON 内容无法解析");
  }
  const value = doc.toJS({}) as unknown;
  if (!isPlainObject(value)) {
    throw new Error("YAML/JSON 必须是对象");
  }
  assertSafeYamlObject(value);
  return value;
}

export function stringifyYamlEditorDocument(value: unknown): string {
  if (!isPlainObject(value)) {
    throw new Error("YAML/JSON 必须是对象");
  }
  assertSafeYamlObject(value);
  return stringify(value);
}

export function assertSafeYamlObject(value: unknown): void {
  visit(value);
}

function visit(value: unknown) {
  if (Array.isArray(value)) {
    for (const item of value) visit(item);
    return;
  }
  if (!value || typeof value !== "object") return;
  for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
    if (DANGEROUS_KEYS.has(key)) {
      throw new Error("YAML/JSON 包含不允许的键");
    }
    visit(child);
  }
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
