import { parseAllDocuments } from "yaml";
import type { GeneratedTemplateArtifact, ProjectVM } from "@sparkops/contracts/workspace";
import { CATALOG, proxyPath, type ResourceDef } from "@/modules/plugins/kubernetes/catalog";
import { k8sFetch } from "@/modules/plugins/kubernetes/api";
import { resourceCollectionUrl } from "@/modules/plugins/kubernetes/resource-url";
import { stringifyYamlEditorDocument } from "@/modules/plugins/kubernetes/yaml-document";

export type ProductK8sSource = "cluster" | "template";

export type ProductK8sResource = {
  id: string;
  projectId: string;
  projectName: string;
  projectLabel: string;
  kind: string;
  name: string;
  yaml: string;
  source: ProductK8sSource;
  clusterId?: string;
  namespace?: string;
  templateKind?: string;
};

const LOAD_KINDS = ["Deployment", "Service"] as const;
const CLUSTER_LIST_TIMEOUT_MS = 8_000;

const KIND_ORDER = new Map(LOAD_KINDS.map((kind, index) => [kind.toLowerCase(), index]));

type K8sMetaObject = {
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
    labels?: Record<string, string>;
  };
};

export function resourceListLabel(kind: string, name: string) {
  return `${kind.trim().toLowerCase()}/${name.trim()}`;
}

export function belongsToProject(object: K8sMetaObject, projectName: string) {
  const name = object.metadata?.name?.trim().toLowerCase() ?? "";
  const labels = object.metadata?.labels ?? {};
  const slug = projectName.trim().toLowerCase();
  if (!slug) return false;
  if (name === slug) return true;
  if (name.startsWith(`${slug}-`) || name.endsWith(`-${slug}`)) return true;
  const app = (labels.app ?? labels["app.kubernetes.io/name"] ?? labels["sparkops.project"] ?? "").toLowerCase();
  return app === slug;
}

export function parseYamlDocuments(text: string): K8sMetaObject[] {
  const docs = parseAllDocuments(text, { prettyErrors: false });
  const result: K8sMetaObject[] = [];
  for (const doc of docs) {
    if (doc.errors.length > 0) continue;
    const value = doc.toJS({}) as unknown;
    if (!value || typeof value !== "object" || Array.isArray(value)) continue;
    result.push(value as K8sMetaObject);
  }
  return result;
}

export function extractLabels(yaml: string): Array<{ key: string; value: string }> {
  const docs = parseYamlDocuments(yaml);
  const labels = docs[0]?.metadata?.labels ?? {};
  return Object.entries(labels)
    .filter(([key, value]) => key && typeof value === "string")
    .map(([key, value]) => ({ key, value }));
}

function stripLiveFields(value: Record<string, unknown>) {
  const next = { ...value };
  delete next.status;
  const metadata = next.metadata;
  if (metadata && typeof metadata === "object" && !Array.isArray(metadata)) {
    const meta = { ...(metadata as Record<string, unknown>) };
    delete meta.managedFields;
    delete meta.resourceVersion;
    delete meta.uid;
    delete meta.generation;
    delete meta.creationTimestamp;
    next.metadata = meta;
  }
  return next;
}

function toYaml(value: unknown) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return "";
  return stringifyYamlEditorDocument(stripLiveFields(value as Record<string, unknown>)).trimEnd();
}

function kindDef(kind: string): ResourceDef | undefined {
  return CATALOG.find((item) => item.kind === kind);
}

function resourceId(projectId: string, source: ProductK8sSource, kind: string, name: string) {
  return `${projectId}:${source}:${kind.toLowerCase()}:${name}`;
}

export function templatesToResources(
  project: ProjectVM,
  artifacts: GeneratedTemplateArtifact[],
): ProductK8sResource[] {
  const items: ProductK8sResource[] = [];
  for (const artifact of artifacts) {
    if (artifact.kind === "jenkinsFile") continue;
    const docs = parseYamlDocuments(artifact.content);
    if (docs.length === 0) {
      const name = project.name;
      const kind = artifact.kind || "Resource";
      items.push({
        id: resourceId(project.id, "template", kind, name),
        projectId: project.id,
        projectName: project.name,
        projectLabel: project.display_name || project.name,
        kind,
        name,
        yaml: artifact.content,
        source: "template",
        templateKind: artifact.kind,
      });
      continue;
    }
    docs.forEach((doc, index) => {
      const kind = String(doc.kind || artifact.kind || "Resource");
      const name = doc.metadata?.name?.trim() || `${project.name}-${index + 1}`;
      let yaml = artifact.content;
      try {
        yaml = toYaml(doc) || artifact.content;
      } catch {
        yaml = artifact.content;
      }
      items.push({
        id: resourceId(project.id, "template", kind, name),
        projectId: project.id,
        projectName: project.name,
        projectLabel: project.display_name || project.name,
        kind,
        name,
        yaml,
        source: "template",
        clusterId: project.default_cluster_id,
        namespace: doc.metadata?.namespace || project.namespace,
        templateKind: artifact.kind,
      });
    });
  }
  return items;
}

async function listKind(
  clusterId: string,
  namespace: string,
  def: ResourceDef,
  signal?: AbortSignal,
): Promise<K8sMetaObject[]> {
  const url = resourceCollectionUrl(clusterId, def, { namespace, page: 1, pageSize: 100 });
  if (!url) return [];
  const data = await k8sFetch<{ items?: K8sMetaObject[] }>(url, { signal });
  return data.items ?? [];
}

function withTimeout<T>(promise: Promise<T>, ms: number, label: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = window.setTimeout(() => reject(new Error(`${label}超时（>${ms}ms）`)), ms);
    promise.then(
      (value) => {
        window.clearTimeout(timer);
        resolve(value);
      },
      (err) => {
        window.clearTimeout(timer);
        reject(err);
      },
    );
  });
}

export async function loadClusterResources(project: ProjectVM): Promise<ProductK8sResource[]> {
  const clusterId = project.default_cluster_id?.trim();
  const namespace = project.namespace?.trim();
  if (!clusterId || !namespace) return [];

  const controller = new AbortController();
  const load = Promise.allSettled(
    LOAD_KINDS.map(async (kind) => {
      const def = kindDef(kind);
      if (!def) return [] as ProductK8sResource[];
      const objects = await listKind(clusterId, namespace, def, controller.signal);
      // 只保留与项目相关的资源，避免把命名空间里无关 ConfigMap/默认账号整页灌进来
      const matched = objects.filter((item) => belongsToProject(item, project.name));
      return matched
        .map((item) => {
          const name = item.metadata?.name?.trim();
          if (!name) return null;
          let yaml = "";
          try {
            yaml = toYaml(item);
          } catch {
            return null;
          }
          return {
            id: resourceId(project.id, "cluster", kind, name),
            projectId: project.id,
            projectName: project.name,
            projectLabel: project.display_name || project.name,
            kind,
            name,
            yaml,
            source: "cluster" as const,
            clusterId,
            namespace: item.metadata?.namespace || namespace,
          };
        })
        .filter((item): item is ProductK8sResource => Boolean(item));
    }),
  ).then((settled) => settled.flatMap((result) => (result.status === "fulfilled" ? result.value : [])));

  try {
    return await withTimeout(load, CLUSTER_LIST_TIMEOUT_MS, "读取集群资源");
  } catch (err) {
    controller.abort();
    throw err;
  }
}

export function mergeProjectResources(clusterItems: ProductK8sResource[], templateItems: ProductK8sResource[]) {
  const keys = new Set(clusterItems.map((item) => `${item.kind.toLowerCase()}:${item.name}`));
  const extras = templateItems.filter((item) => !keys.has(`${item.kind.toLowerCase()}:${item.name}`));
  return sortResources([...clusterItems, ...extras]);
}

export function sortResources(items: ProductK8sResource[]) {
  return [...items].sort((a, b) => {
    const project = a.projectLabel.localeCompare(b.projectLabel, "zh");
    if (project !== 0) return project;
    const kind =
      (KIND_ORDER.get(a.kind.toLowerCase()) ?? 99) - (KIND_ORDER.get(b.kind.toLowerCase()) ?? 99);
    if (kind !== 0) return kind;
    return a.name.localeCompare(b.name, "zh");
  });
}

export function applyNamespaceToYaml(yaml: string, namespace: string) {
  const docs = parseYamlDocuments(yaml);
  if (docs.length === 0) return yaml;
  const next = docs.map((doc) => {
    const metadata = { ...(doc.metadata ?? {}), namespace };
    return { ...doc, metadata };
  });
  return next
    .map((doc) => toYaml(doc))
    .filter(Boolean)
    .join("\n---\n");
}

export async function applyResourceToCluster(resource: ProductK8sResource, clusterId: string, namespace: string) {
  const def = kindDef(resource.kind);
  if (!def) throw new Error(`暂不支持将 ${resource.kind} 加入环境`);
  const docs = parseYamlDocuments(applyNamespaceToYaml(resource.yaml, namespace));
  if (docs.length === 0) throw new Error("YAML 无法解析，请检查内容");
  for (const doc of docs) {
    const name = doc.metadata?.name?.trim();
    if (!name) throw new Error("YAML 缺少 metadata.name");
    const body = JSON.stringify({ ...doc, metadata: { ...doc.metadata, namespace, name } });
    const itemPath = proxyPath(clusterId, def, namespace, name);
    try {
      await k8sFetch(itemPath);
      await k8sFetch(itemPath, { method: "PUT", body });
    } catch {
      await k8sFetch(proxyPath(clusterId, def, namespace), { method: "POST", body });
    }
  }
}
