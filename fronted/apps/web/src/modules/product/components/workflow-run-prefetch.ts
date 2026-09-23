import type { EnvironmentVM } from "@sparkops/contracts/product";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { product, secret, workspace } from "@sparkops/web-sdk";

export interface WorkspaceRunAssets {
  gitProjects: ProjectVM[];
  registries: SecretVM[];
}

const workspaceAssetsCache = new Map<string, WorkspaceRunAssets>();
const workspaceAssetsInflight = new Map<string, Promise<WorkspaceRunAssets>>();
const environmentsCache = new Map<string, EnvironmentVM[]>();
const environmentsInflight = new Map<string, Promise<EnvironmentVM[]>>();

function envKey(workspaceId: string, productId: string) {
  return `${workspaceId}:${productId}`;
}

async function loadWorkspaceRunAssets(workspaceId: string): Promise<WorkspaceRunAssets> {
  const [projectItems, secretItems] = await Promise.all([
    workspace.listProjects(workspaceId),
    secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
  ]);
  return {
    gitProjects: projectItems,
    registries: secretItems.filter((item) => {
      const type = item.service_type.toLowerCase();
      return type.includes("harbor") || type.includes("registry") || type.includes("docker");
    }),
  };
}

export function peekWorkspaceRunAssets(workspaceId: string): WorkspaceRunAssets | undefined {
  return workspaceAssetsCache.get(workspaceId);
}

export function prefetchWorkspaceRunAssets(workspaceId: string): Promise<WorkspaceRunAssets> {
  const cached = workspaceAssetsCache.get(workspaceId);
  if (cached) return Promise.resolve(cached);

  let inflight = workspaceAssetsInflight.get(workspaceId);
  if (!inflight) {
    inflight = loadWorkspaceRunAssets(workspaceId).then((assets) => {
      workspaceAssetsCache.set(workspaceId, assets);
      workspaceAssetsInflight.delete(workspaceId);
      return assets;
    });
    workspaceAssetsInflight.set(workspaceId, inflight);
  }
  return inflight;
}

export function peekProductEnvironments(workspaceId: string, productId: string): EnvironmentVM[] | undefined {
  return environmentsCache.get(envKey(workspaceId, productId));
}

export function prefetchProductEnvironments(workspaceId: string, productId: string): Promise<EnvironmentVM[]> {
  const key = envKey(workspaceId, productId);
  const cached = environmentsCache.get(key);
  if (cached) return Promise.resolve(cached);

  let inflight = environmentsInflight.get(key);
  if (!inflight) {
    inflight = product
      .listEnvironments(workspaceId, productId)
      .then((items) => {
        environmentsCache.set(key, items);
        environmentsInflight.delete(key);
        return items;
      })
      .catch(() => {
        environmentsInflight.delete(key);
        return [] as EnvironmentVM[];
      });
    environmentsInflight.set(key, inflight);
  }
  return inflight;
}

export function prefetchWorkflowRunContext(
  workspaceId: string,
  productId: string,
  workflowId: string,
): void {
  prefetchWorkspaceRunAssets(workspaceId);
  prefetchProductEnvironments(workspaceId, productId);
  void product.getProductWorkflow(workspaceId, productId, workflowId).catch(() => undefined);
}
