import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";

export type EnvironmentSuggestCandidate = {
  key: string;
  name: string;
  env_type: "test" | "production";
  cluster_id: string;
  cluster_name: string;
  namespace: string;
  project_ids: string[];
  project_labels: string[];
};

function looksProduction(namespace: string) {
  const ns = namespace.toLowerCase();
  return ns.includes("prod") || ns.includes("production");
}

function defaultEnvName(namespace: string, index: number, total: number) {
  const ns = namespace.trim().toLowerCase();
  if (!ns) return total === 1 ? "dev" : `env-${index + 1}`;
  if (ns.includes("prod")) return "prod";
  if (ns.includes("uat")) return "uat";
  if (ns.includes("sit")) return "sit";
  if (ns.includes("dev") || ns.includes("test")) return "dev";
  if (total === 1) return "dev";
  return ns.slice(0, 32);
}

/** 从产品已绑定项目的 cluster+namespace 归纳可创建的环境候选 */
export function suggestEnvironmentsFromProjects(
  productProjects: Array<{ id: string }>,
  workspaceProjects: ProjectVM[],
  clusters: AuthorizedClusterVM[],
): EnvironmentSuggestCandidate[] {
  const boundIds = new Set(productProjects.map((item) => item.id));
  const deployClusters = clusters.filter((item) => !item.is_build_env);
  const clusterNameById = new Map(deployClusters.map((item) => [item.id, item.cluster_name]));

  const groups = new Map<
    string,
    { cluster_id: string; namespace: string; project_ids: string[]; project_labels: string[] }
  >();

  for (const project of workspaceProjects) {
    if (!boundIds.has(project.id) || project.status !== "active") continue;
    const clusterId = project.default_cluster_id?.trim() ?? "";
    const namespace = project.namespace?.trim() ?? "";
    if (!clusterId || !namespace) continue;
    if (!clusterNameById.has(clusterId)) continue;
    const key = `${clusterId}::${namespace}`;
    const current = groups.get(key) ?? {
      cluster_id: clusterId,
      namespace,
      project_ids: [],
      project_labels: [],
    };
    current.project_ids.push(project.id);
    current.project_labels.push(project.display_name || project.name);
    groups.set(key, current);
  }

  const values = [...groups.values()];
  return values.map((group, index) => ({
    key: `${group.cluster_id}::${group.namespace}`,
    name: defaultEnvName(group.namespace, index, values.length),
    env_type: looksProduction(group.namespace) ? "production" : "test",
    cluster_id: group.cluster_id,
    cluster_name: clusterNameById.get(group.cluster_id) ?? group.cluster_id,
    namespace: group.namespace,
    project_ids: group.project_ids,
    project_labels: group.project_labels,
  }));
}

export function deployableAuthorizedClusters(clusters: AuthorizedClusterVM[]) {
  return [...clusters]
    .filter((item) => !item.is_build_env && item.available !== false)
    .sort((a, b) => (a.priority_id ?? 0) - (b.priority_id ?? 0));
}

/** 对齐 Zadig 默认规则：产品名-env-环境名 */
export function suggestEnvironmentNamespace(productSlug: string, envName: string): string {
  const product = productSlug
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  const env = envName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  if (!product && !env) return "";
  if (!product) return env;
  if (!env) return `${product}-env`;
  return `${product}-env-${env}`;
}
