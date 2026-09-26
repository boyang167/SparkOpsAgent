import type { WorkflowNotificationVM } from "@sparkops/contracts/product";
import { toNotifyItems, type NotifyItem } from "./workflow-notify";

export const DEPLOY_CONTENTS = [
  { id: "image", label: "服务镜像" },
  { id: "variables", label: "服务变量" },
  { id: "config", label: "服务配置" },
] as const;

export type DeployArtifactSource = "build" | "version";
export type DeployVersionSourceKind = "auto" | "build" | "create-version";

export interface DeployJobSpec {
  artifactSource: DeployArtifactSource;
  sourceKind: DeployVersionSourceKind;
  environmentId: string;
  jobName: string;
  contents: string[];
  statusCheck: boolean;
  timeoutMinutes: number;
  notify: NotifyItem[];
  configResources: ConfigResourceRef[];
  configResourcesSet: boolean;
}

export interface ConfigResourceRef {
  projectId: string;
  kind: "ConfigMap" | "Secret";
  name: string;
  yaml?: string;
}

export interface ConfigResourceItem extends ConfigResourceRef {
  reason: string;
  referenced: boolean;
  yaml: string;
  note: string;
}

export interface SourceImageLine {
  projectId: string;
  name: string;
  image: string;
  complete: boolean;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function parseConfigResources(value: unknown): ConfigResourceRef[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    const row = asRecord(item);
    const projectId = asString(row?.project_id);
    const kind = asString(row?.kind);
    const name = asString(row?.name);
    if (!projectId || !name || (kind !== "ConfigMap" && kind !== "Secret")) return [];
    return [{ projectId, kind, name, yaml: asString(row?.yaml) }];
  });
}

export function isReferencedReason(reason: string): boolean {
  const value = reason.trim().toLowerCase();
  return value.startsWith("referenced by") || value.startsWith("image pull");
}

export function configReasonLabel(reason: string): string {
  const labels: Record<string, string> = {
    "referenced by Deployment volume": "被 Deployment 卷引用",
    "referenced by Deployment envFrom": "被 Deployment envFrom 引用",
    "referenced by Deployment env": "被 Deployment 环境变量引用",
    "image pull secret": "镜像拉取",
    "name prefix": "名称关联，默认不同步",
    "label match": "标签关联，默认不同步",
  };
  return labels[reason] || (reason.trim() ? reason : "未关联，默认不同步");
}

export function isConfigResourceSelected(selected: ConfigResourceRef[], item: ConfigResourceRef): boolean {
  return selected.some((row) => row.projectId === item.projectId && row.kind === item.kind && row.name === item.name);
}

export function defaultConfigResources(items: ConfigResourceItem[]): ConfigResourceRef[] {
  return items.filter((item) => item.referenced).map(({ projectId, kind, name, yaml }) => ({ projectId, kind, name, yaml }));
}

export function buildConfigResourceItems(
  projectId: string,
  maps: { name: string; namespace?: string; data?: Record<string, string> }[],
  previewItems: { kind: string; name: string; related_reason?: string; yaml?: string; note?: string }[] | null,
): ConfigResourceItem[] {
  const byKey = new Map<string, ConfigResourceItem>();
  for (const map of maps) {
    if (!map.name.trim()) continue;
    byKey.set(`ConfigMap/${map.name}`, {
      projectId,
      kind: "ConfigMap",
      name: map.name,
      reason: "",
      referenced: false,
      yaml: configMapPreviewYaml(map.name, map.namespace ?? "", map.data),
      note: "",
    });
  }
  for (const item of previewItems ?? []) {
    if (item.kind !== "ConfigMap" && item.kind !== "Secret") continue;
    if (!item.name.trim()) continue;
    const key = `${item.kind}/${item.name}`;
    const existing = byKey.get(key);
    const reason = item.related_reason ?? "";
    byKey.set(key, {
      projectId,
      kind: item.kind,
      name: item.name,
      reason,
      referenced: isReferencedReason(reason),
      yaml: item.yaml?.trim() || existing?.yaml || "",
      note: item.note?.trim() ?? "",
    });
  }
  return [...byKey.values()].sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === "ConfigMap" ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
}

function configMapPreviewYaml(name: string, namespace: string, data: Record<string, string> | undefined): string {
  const lines = ["apiVersion: v1", "kind: ConfigMap", "metadata:", `  name: ${name}`];
  if (namespace.trim()) lines.push(`  namespace: ${namespace.trim()}`);
  const entries = Object.entries(data ?? {});
  if (entries.length === 0) {
    lines.push("data: {}");
    return lines.join("\n");
  }
  lines.push("data:");
  for (const [key, value] of entries) {
    const text = String(value ?? "");
    if (text.includes("\n") || text.includes(":") || text.includes("#")) {
      lines.push(`  ${key}: |`);
      for (const row of text.split("\n")) lines.push(`    ${row}`);
    } else {
      lines.push(`  ${key}: ${text}`);
    }
  }
  return lines.join("\n");
}

export function parseDeployJobSpec(
  spec: Record<string, unknown> | undefined,
  jobType?: string,
): DeployJobSpec {
  const artifactSource: DeployArtifactSource =
    jobType === "sparkops-deploy-version" || asString(spec?.artifact_source) === "version" ? "version" : "build";
  const rawSourceKind = asString(spec?.source_kind);
  const sourceKind: DeployVersionSourceKind =
    rawSourceKind === "build" || rawSourceKind === "create-version" ? rawSourceKind : "auto";
  const contents = Array.isArray(spec?.deploy_contents)
    ? spec.deploy_contents.filter((item): item is string => typeof item === "string" && DEPLOY_CONTENTS.some((row) => row.id === item))
    : [];
  const strategy = asRecord(spec?.strategy);
  const timeout = Number(strategy?.timeout_minutes);
  const notify = Array.isArray(spec?.notify) ? toNotifyItems(spec.notify as WorkflowNotificationVM[]) : [];
  return {
    artifactSource,
    sourceKind,
    environmentId: asString(spec?.environment_id),
    jobName: asString(spec?.job_name),
    contents: contents.length > 0 ? contents : ["image"],
    statusCheck: spec?.status_check !== false,
    timeoutMinutes: Number.isFinite(timeout) && timeout > 0 ? timeout : 60,
    notify,
    configResources: parseConfigResources(spec?.config_resources),
    configResourcesSet: Array.isArray(spec?.config_resources),
  };
}

export function deployJobSpecToRecord(spec: DeployJobSpec): Record<string, unknown> {
  const base: Record<string, unknown> = {
    artifact_source: spec.artifactSource,
    ...(spec.artifactSource === "version" && spec.sourceKind !== "auto"
      ? { source_kind: spec.sourceKind }
      : {}),
    environment_id: spec.environmentId,
    source: "fromjob",
    job_name: spec.jobName,
    status_check: spec.statusCheck,
    strategy: { timeout_minutes: spec.timeoutMinutes },
    notify: spec.notify,
  };
  if (spec.artifactSource === "build") {
    base.deploy_contents = spec.contents;
    if (spec.configResourcesSet) {
      base.config_resources = spec.configResources.map((item) => ({
        project_id: item.projectId,
        kind: item.kind,
        name: item.name,
        yaml: item.yaml ?? "",
      }));
    }
  }
  return base;
}

export function formatImageRef(repository?: string, tag?: string): string {
  const repo = repository?.trim();
  const imageTag = tag?.trim();
  if (repo && imageTag) return `${repo}:${imageTag}`;
  if (repo) return repo;
  return imageTag || "—";
}

export function imageTagFromRef(ref: string): string {
  let value = ref.trim().replace(/^refs\/heads\//, "").replace(/^refs\/tags\//, "");
  if (!value) return "latest";
  value = Array.from(value)
    .map((char) => (/[a-zA-Z0-9._-]/.test(char) ? char : "-"))
    .join("")
    .replace(/^[.\-_]+|[.\-_]+$/g, "");
  if (!value) return "latest";
  return value.slice(0, 255);
}

export function deployImageRef(
  buildSpec: Record<string, unknown> | undefined,
  projectId: string,
  branch: string,
  registries: { id: string; serviceUrl: string }[],
  projects: Array<{ id: string; name?: string; namespace?: string; displayName?: string }>,
): string {
  const record = asRecord(buildSpec);
  const registryId = asString(record?.registry_id);
  const registry = registries.find((item) => item.id === registryId);
  const project = projects.find((entry) => entry.id === projectId);
  const projectName = project?.name || projectId;
  const tag = imageTagFromRef(branch);
  const repository = imageRepositoryPath(registry?.serviceUrl ?? "", project?.namespace ?? "", projectName);
  if (repository && repository.includes("/")) return `${repository}:${tag}`;
  if (projectName) return `${projectName.toLowerCase()}:${tag}`;
  return tag;
}

export function imageRepositoryPath(registryURL: string, namespace: string, projectName: string): string {
  const raw = registryURL.trim();
  if (!raw) return "";
  const parseable = raw.includes("://") ? raw : `https://${raw}`;
  let host = "";
  try {
    host = new URL(parseable).host;
  } catch {
    host = "";
  }
  if (!host) host = raw.replace(/^\/+|\/+$/g, "");
  const ns = namespace.trim().toLowerCase().replace(/^\/+|\/+$/g, "");
  const name = projectName.trim().toLowerCase().replace(/^\/+|\/+$/g, "");
  const base = host.toLowerCase().replace(/^\/+|\/+$/g, "");
  if (!ns || !name) return base;
  return `${base}/${ns}/${name}`;
}

export function sourceImageLines(
  spec: Record<string, unknown> | undefined,
  registries: { id: string; serviceUrl: string }[],
  projects: { id: string; name: string; namespace?: string; displayName?: string }[],
): SourceImageLine[] {
  const record = asRecord(spec);
  const registryId = asString(record?.registry_id);
  const registry = registries.find((item) => item.id === registryId);
  const rows = Array.isArray(record?.project_builds) ? record.project_builds : [];
  return rows.flatMap((item) => {
    const row = asRecord(item);
    if (!row) return [];
    const projectId = asString(row.project_id);
    if (!projectId) return [];
    const project = projects.find((entry) => entry.id === projectId);
    const projectName = project?.name || projectId;
    const branch = asString(row.branch) || "main";
    const tag = imageTagFromRef(branch);
    const repository = imageRepositoryPath(registry?.serviceUrl ?? "", project?.namespace ?? "", projectName);
    const complete = Boolean(repository && repository.includes("/"));
    return [
      {
        projectId,
        name: project?.displayName || projectName,
        image: complete ? `${repository}:${tag}` : `${projectName.toLowerCase()}:${tag}`,
        complete,
      },
    ];
  });
}
