import type { BuildVM } from "@sparkops/contracts/delivery";
import type { ProductVM } from "@sparkops/contracts/product";
import { agent, delivery, knowledge, platform, product, workflow, workspace } from "@sparkops/web-sdk";
import type { CommandSearchItem } from "@/components/layout/command-search-index";
import { isPathEntitled } from "@/components/layout/entitlements";

const MAX_PROJECTS = 12;
const MAX_BUILDS_PER_PROJECT = 5;
const MAX_PRODUCTS = 20;
const MAX_CLUSTERS = 15;

export async function buildDynamicCommandItems(options: {
  workspaceId: string;
  isSuperAdmin: boolean;
  entitlements: import("@sparkops/contracts/platform").WorkspaceEntitlementsVM;
}): Promise<CommandSearchItem[]> {
  const { workspaceId, isSuperAdmin, entitlements } = options;
  const allow = (href: string) => isPathEntitled(href, entitlements);

  const projects = await workspace.listProjects(workspaceId).catch(() => []);
  const limitedProjects = projects.slice(0, MAX_PROJECTS);

  const [knowledgeBases, products, clusters] = await Promise.all([
    knowledge.listKnowledgeBases(workspaceId).catch(() => []),
    entitlements.delivery
      ? product.listProducts(workspaceId).catch(() => [] as ProductVM[])
      : Promise.resolve([] as ProductVM[]),
    isSuperAdmin && entitlements.k8s
      ? platform.listClusters().catch(() => [] as Array<Record<string, unknown>>)
      : Promise.resolve([] as Array<Record<string, unknown>>),
  ]);

  const projectItems: CommandSearchItem[] = projects.flatMap((project) =>
    [
      {
        href: `/projects/${project.id}`,
        label: project.name,
        group: "项目",
        kind: "project" as const,
        keywords: `${project.name} ${project.description ?? ""}`,
      },
      {
        href: `/projects/${project.id}/agents`,
        label: `${project.name} · 智能体`,
        group: "项目",
        kind: "project" as const,
        keywords: `${project.name} agents 智能体`,
      },
      {
        href: `/projects/${project.id}/workflows`,
        label: `${project.name} · 工作流`,
        group: "项目",
        kind: "project" as const,
        keywords: `${project.name} workflow 工作流`,
      },
    ].filter((item) => allow(item.href)),
  );

  const kbItems: CommandSearchItem[] = knowledgeBases.map((kb) => ({
    href: `/knowledge/${kb.id}`,
    label: kb.name,
    group: "知识库",
    kind: "knowledge" as const,
    keywords: kb.description ?? "",
  }));

  const productItems: CommandSearchItem[] = products.slice(0, MAX_PRODUCTS).map((item) => ({
    href: "/publish/products",
    label: item.name,
    group: "持续交付",
    kind: "product" as const,
    keywords: `${item.name} 产品 product`,
  }));

  const clusterItems: CommandSearchItem[] = clusters.slice(0, MAX_CLUSTERS).map((cluster) => {
    const name = String(cluster.cluster_name ?? cluster.name ?? "集群");
    return {
      href: "/platform/clusters",
      label: name,
      group: "平台运营",
      kind: "cluster" as const,
      keywords: `${name} 集群 cluster`,
    };
  });

  const nested = await Promise.all(
    limitedProjects.map(async (project) => {
      const [agents, workflows, builds] = await Promise.all([
        agent.listAgents(workspaceId, project.id).catch(() => []),
        workflow.listWorkflows(workspaceId, project.id).catch(() => []),
        entitlements.delivery
          ? delivery.listBuilds(project.id, workspaceId).catch(() => [] as BuildVM[])
          : Promise.resolve([] as BuildVM[]),
      ]);

      const agentItems: CommandSearchItem[] = agents.map((item) => ({
        href: `/projects/${project.id}/agents/${item.id}`,
        label: item.name,
        group: project.name,
        kind: "agent" as const,
        keywords: `${project.name} agent 智能体`,
      }));

      const workflowItems: CommandSearchItem[] = workflows.map((item) => ({
        href: `/projects/${project.id}/workflows/${item.id}`,
        label: item.name,
        group: project.name,
        kind: "workflow" as const,
        keywords: `${project.name} workflow 工作流`,
      }));

      const buildItems: CommandSearchItem[] = builds.slice(0, MAX_BUILDS_PER_PROJECT).map((item) => ({
        href: `/delivery/builds/${item.id}`,
        label: `${project.name} · 构建 #${item.build_number}`,
        group: "持续交付",
        kind: "build" as const,
        keywords: `${project.name} build 构建 ${item.branch ?? ""}`,
      }));

      return [...agentItems, ...workflowItems, ...buildItems].filter((item) => allow(item.href));
    }),
  );

  return [
    ...projectItems,
    ...kbItems,
    ...productItems,
    ...clusterItems,
    ...nested.flat(),
  ].filter((item) => allow(item.href));
}
