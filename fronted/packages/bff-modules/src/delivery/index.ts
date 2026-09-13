import type { BffContext } from "@sparkops/bff-core";
import type {
  BuildVM,
  CreateDeploymentCommand,
  DeploymentOptionsVM,
  DeploymentRunListVM,
  DeploymentRunVM,
  RollbackDeploymentCommand,
  RollbackPreviewVM,
  SyncPreviewVM,
  TriggerBuildCommand,
} from "@sparkops/contracts/delivery";

export async function listBuilds(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<BuildVM[]> {
  const data = await ctx.upstream.delivery.listBuilds(workspaceId, projectId);
  return data.items;
}

export async function triggerBuild(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: TriggerBuildCommand,
): Promise<BuildVM> {
  return ctx.upstream.delivery.triggerBuild(workspaceId, projectId, input.branch);
}

export async function getBuild(
  ctx: BffContext,
  workspaceId: string,
  buildId: string,
): Promise<BuildVM> {
  return ctx.upstream.delivery.getBuild(workspaceId, buildId);
}

export async function abortBuild(
  ctx: BffContext,
  workspaceId: string,
  buildId: string,
): Promise<{ aborted: true }> {
  return ctx.upstream.delivery.abortBuild(workspaceId, buildId);
}

export type RollbackBuildResult = {
  id: string;
  status: string;
  build_number: number;
  rollback_of_build_id: string;
};

export async function rollbackBuild(
  ctx: BffContext,
  workspaceId: string,
  buildId: string,
): Promise<RollbackBuildResult> {
  return ctx.upstream.delivery.rollbackBuild(workspaceId, buildId);
}

export async function listDeployments(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<DeploymentRunListVM> {
  return ctx.upstream.delivery.listDeployments(workspaceId, projectId);
}

export async function createDeployment(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: CreateDeploymentCommand,
): Promise<DeploymentRunVM> {
  return ctx.upstream.delivery.createDeployment(workspaceId, projectId, input);
}

export async function rollbackDeployment(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: RollbackDeploymentCommand = {},
): Promise<DeploymentRunVM> {
  return ctx.upstream.delivery.rollbackDeployment(workspaceId, projectId, input);
}

export async function getRollbackPreview(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  targetRunId?: string,
): Promise<RollbackPreviewVM> {
  return ctx.upstream.delivery.getRollbackPreview(workspaceId, projectId, targetRunId);
}

export async function getSyncPreview(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  imageTag?: string,
): Promise<SyncPreviewVM> {
  return ctx.upstream.delivery.getSyncPreview(workspaceId, projectId, imageTag);
}

export async function getDeployment(
  ctx: BffContext,
  workspaceId: string,
  runId: string,
): Promise<DeploymentRunVM> {
  return ctx.upstream.delivery.getDeployment(workspaceId, runId);
}

export async function abortDeployment(
  ctx: BffContext,
  workspaceId: string,
  runId: string,
): Promise<DeploymentRunVM> {
  return ctx.upstream.delivery.abortDeployment(workspaceId, runId);
}

export async function getDeploymentOptions(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<DeploymentOptionsVM> {
  return ctx.upstream.delivery.getDeploymentOptions(workspaceId, projectId);
}

export async function listCronJobs(ctx: BffContext, workspaceId: string, projectId: string) {
  const data = await ctx.upstream.delivery.listCronJobs(workspaceId, projectId);
  return data.items;
}

export async function createCronJob(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: { name: string; schedule: string; branch?: string },
) {
  return ctx.upstream.delivery.createCronJob(workspaceId, projectId, input);
}

export async function deleteCronJob(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  cronJobId: string,
) {
  return ctx.upstream.delivery.deleteCronJob(workspaceId, projectId, cronJobId);
}

export async function listWebhooks(ctx: BffContext, workspaceId: string, projectId: string) {
  return ctx.upstream.delivery.listWebhooks(workspaceId, projectId);
}

export async function createWebhook(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: import("@sparkops/contracts/delivery").UpsertProjectWebhookInput,
) {
  return ctx.upstream.delivery.createWebhook(workspaceId, projectId, input);
}

export async function updateWebhook(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  webhookId: string,
  input: import("@sparkops/contracts/delivery").UpsertProjectWebhookInput,
) {
  return ctx.upstream.delivery.updateWebhook(workspaceId, projectId, webhookId, input);
}

export async function deleteWebhook(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  webhookId: string,
) {
  return ctx.upstream.delivery.deleteWebhook(workspaceId, projectId, webhookId);
}
