import type { BffContext } from "@sparkops/bff-core";

export async function listProducts(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listProducts(workspaceId);
  return data.items;
}

export function createProduct(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateProductCommand,
) {
  return ctx.upstream.product.createProduct(workspaceId, input);
}

export async function getProduct(ctx: BffContext, workspaceId: string, productId: string) {
  return ctx.upstream.product.getProduct(workspaceId, productId);
}

export async function getProductOverview(ctx: BffContext, workspaceId: string, productId: string) {
  return ctx.upstream.product.getProductOverview(workspaceId, productId);
}

export async function listProductProjects(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listProductProjects(workspaceId, productId);
  return data.items;
}

export async function listProductPlans(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listProductPlans(workspaceId, productId);
  return data.items;
}

export {
  listRequirements,
  createRequirement,
  approveRequirement,
  rejectRequirement,
  releaseRequirement,
} from "./requirements";

export async function listTemplates(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listTemplates(workspaceId);
  return data.items;
}

export async function getTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.getTemplate(workspaceId, templateId);
}

export async function createTemplate(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateTemplateCommand,
) {
  return ctx.upstream.product.createTemplate(workspaceId, input);
}

export async function updateTemplate(
  ctx: BffContext,
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").UpdateTemplateCommand,
) {
  return ctx.upstream.product.updateTemplate(workspaceId, templateId, input);
}

export async function deleteTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.deleteTemplate(workspaceId, templateId);
}

export async function listWorkflowTemplates(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listWorkflowTemplates(workspaceId);
  return data.items;
}

export async function getWorkflowTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.getWorkflowTemplate(workspaceId, templateId);
}

export async function previewWorkflowTemplate(
  ctx: BffContext,
  workspaceId: string,
  templateId: string,
  keyVals: Record<string, string>,
) {
  return ctx.upstream.product.previewWorkflowTemplate(workspaceId, templateId, keyVals);
}

export function createWorkflowTemplate(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateWorkflowTemplateCommand,
) {
  return ctx.upstream.product.createWorkflowTemplate(workspaceId, input);
}

export function updateWorkflowTemplate(
  ctx: BffContext,
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").UpdateWorkflowTemplateCommand,
) {
  return ctx.upstream.product.updateWorkflowTemplate(workspaceId, templateId, input);
}

export function publishWorkflowTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.publishWorkflowTemplate(workspaceId, templateId);
}

export async function listWorkspaceWorkflows(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listWorkspaceWorkflows(workspaceId);
  return data.items;
}

export async function listReleasePlans(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  sprintId?: string,
) {
  const data = await ctx.upstream.product.listReleasePlans(workspaceId, productId, sprintId);
  return data.items;
}

export async function listReleasePlansPage(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  query: { sprintId?: string; q?: string; limit?: number; offset?: number },
) {
  const { sprintId, ...page } = query;
  const data = await ctx.upstream.product.listReleasePlans(workspaceId, productId, sprintId, page);
  return { items: data.items, has_more: data.has_more ?? false };
}

export async function getReleasePlan(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.getReleasePlan(workspaceId, productId, planId);
}

export function listReleasePlanVersions(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.listReleasePlanVersions(workspaceId, productId, planId);
}

export function getReleasePlanVersionDiff(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  version: number,
  compareVersion?: number,
) {
  return ctx.upstream.product.getReleasePlanVersionDiff(workspaceId, productId, planId, version, compareVersion);
}

export async function createReleasePlan(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateReleasePlanCommand,
) {
  return ctx.upstream.product.createReleasePlan(workspaceId, productId, input);
}

export async function addReleasePlanJob(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  input: import("@sparkops/contracts/product").AddReleasePlanJobCommand,
) {
  return ctx.upstream.product.addReleasePlanJob(workspaceId, productId, planId, input);
}

export function finishReleasePlanPlanning(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.finishReleasePlanPlanning(workspaceId, productId, planId);
}

export function approveReleasePlan(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.approveReleasePlan(workspaceId, productId, planId);
}

export function markReleasePlanJobDone(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  jobId: string,
) {
  return ctx.upstream.product.markReleasePlanJobDone(workspaceId, productId, planId, jobId);
}

export function skipReleasePlanJob(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  jobId: string,
) {
  return ctx.upstream.product.skipReleasePlanJob(workspaceId, productId, planId, jobId);
}

export function executeReleasePlan(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.executeReleasePlan(workspaceId, productId, planId);
}

export function retryReleasePlanJob(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  jobId: string,
) {
  return ctx.upstream.product.retryReleasePlanJob(workspaceId, productId, planId, jobId);
}

export async function listWorkspaceReleasePlans(
  ctx: BffContext,
  workspaceId: string,
  query?: { status?: string; from?: string; to?: string },
) {
  const data = await ctx.upstream.product.listWorkspaceReleasePlans(workspaceId, query);
  return data.items;
}

export function updateReleasePlan(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  input: import("@sparkops/contracts/product").UpdateReleasePlanCommand,
) {
  return ctx.upstream.product.updateReleasePlan(workspaceId, productId, planId, input);
}

export function cancelReleasePlan(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.cancelReleasePlan(workspaceId, productId, planId);
}

export function deleteReleasePlan(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.deleteReleasePlan(workspaceId, productId, planId);
}

export function reopenReleasePlanPlanning(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.reopenReleasePlanPlanning(workspaceId, productId, planId);
}

export function rejectReleasePlan(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  input: import("@sparkops/contracts/product").RejectReleasePlanCommand,
) {
  return ctx.upstream.product.rejectReleasePlan(workspaceId, productId, planId, input);
}

export function reorderReleasePlanJobs(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  jobIds: string[],
) {
  return ctx.upstream.product.reorderReleasePlanJobs(workspaceId, productId, planId, jobIds);
}

export function updateReleasePlanJob(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  jobId: string,
  input: import("@sparkops/contracts/product").UpdateReleasePlanJobCommand,
) {
  return ctx.upstream.product.updateReleasePlanJob(workspaceId, productId, planId, jobId, input);
}

export function deleteReleasePlanJob(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  jobId: string,
) {
  return ctx.upstream.product.deleteReleasePlanJob(workspaceId, productId, planId, jobId);
}

export function addReleasePlanJobsFromWorkItems(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  workItemIds: string[],
) {
  return ctx.upstream.product.addReleasePlanJobsFromWorkItems(workspaceId, productId, planId, workItemIds);
}

export function copyReleasePlan(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  input: import("@sparkops/contracts/product").CopyReleasePlanCommand,
) {
  return ctx.upstream.product.copyReleasePlan(workspaceId, productId, planId, input);
}

export function saveReleasePlanTemplate(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
  input: import("@sparkops/contracts/product").SaveReleasePlanTemplateCommand,
) {
  return ctx.upstream.product.saveReleasePlanTemplate(workspaceId, productId, planId, input);
}

export async function listReleasePlanTemplates(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listReleasePlanTemplates(workspaceId);
  return data.items;
}

export function createReleasePlanFromTemplate(
  ctx: BffContext,
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").CreateReleasePlanFromTemplateCommand,
) {
  return ctx.upstream.product.createReleasePlanFromTemplate(workspaceId, templateId, input);
}

export function deleteReleasePlanTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.deleteReleasePlanTemplate(workspaceId, templateId);
}

export async function listProductWorkflows(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listProductWorkflows(workspaceId, productId);
  return data.items;
}

export function getProductWorkflow(ctx: BffContext, workspaceId: string, productId: string, workflowId: string) {
  return ctx.upstream.product.getProductWorkflow(workspaceId, productId, workflowId);
}

export function createBlankProductWorkflow(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateBlankProductWorkflowCommand,
) {
  return ctx.upstream.product.createBlankProductWorkflow(workspaceId, productId, input);
}

export function updateProductWorkflow(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  input: import("@sparkops/contracts/product").UpdateProductWorkflowCommand,
) {
  return ctx.upstream.product.updateProductWorkflow(workspaceId, productId, workflowId, input);
}

export function deleteProductWorkflow(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
) {
  return ctx.upstream.product.deleteProductWorkflow(workspaceId, productId, workflowId);
}

export function createProductWorkflowFromTemplate(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateProductWorkflowCommand,
) {
  return ctx.upstream.product.createProductWorkflowFromTemplate(workspaceId, productId, input);
}

export function startProductWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  input?: import("@sparkops/contracts/product").StartProductWorkflowRunCommand,
) {
  return ctx.upstream.product.startProductWorkflowRun(workspaceId, productId, workflowId, input);
}

export async function listProductWorkflowRuns(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
) {
  const data = await ctx.upstream.product.listProductWorkflowRuns(workspaceId, productId, workflowId);
  return data.items;
}

export function getProductWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
) {
  return ctx.upstream.product.getProductWorkflowRun(workspaceId, productId, workflowId, runId);
}

export function updateProductWorkflowRunRemark(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  input: import("@sparkops/contracts/product").UpdateProductWorkflowRunRemarkCommand,
) {
  return ctx.upstream.product.updateProductWorkflowRunRemark(workspaceId, productId, workflowId, runId, input);
}

export function approveWorkflowJob(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  jobName: string,
) {
  return ctx.upstream.product.approveWorkflowJob(workspaceId, productId, workflowId, runId, jobName);
}

export function rejectWorkflowJob(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  jobName: string,
  input?: import("@sparkops/contracts/product").RejectWorkflowJobCommand,
) {
  return ctx.upstream.product.rejectWorkflowJob(workspaceId, productId, workflowId, runId, jobName, input);
}

export function continueProductWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
) {
  return ctx.upstream.product.continueProductWorkflowRun(workspaceId, productId, workflowId, runId);
}

export function pauseProductWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
) {
  return ctx.upstream.product.pauseProductWorkflowRun(workspaceId, productId, workflowId, runId);
}

export function cancelProductWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  input?: import("@sparkops/contracts/product").CancelProductWorkflowRunCommand,
) {
  return ctx.upstream.product.cancelProductWorkflowRun(workspaceId, productId, workflowId, runId, input);
}

export function retryProductWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
) {
  return ctx.upstream.product.retryProductWorkflowRun(workspaceId, productId, workflowId, runId);
}

export async function listEnvironments(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listEnvironments(workspaceId, productId);
  return data.items;
}

export function createEnvironment(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateEnvironmentCommand,
) {
  return ctx.upstream.product.createEnvironment(workspaceId, productId, input);
}

export function updateEnvironment(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  envId: string,
  input: import("@sparkops/contracts/product").UpdateEnvironmentCommand,
) {
  return ctx.upstream.product.updateEnvironment(workspaceId, productId, envId, input);
}

export function deleteEnvironment(ctx: BffContext, workspaceId: string, productId: string, envId: string) {
  return ctx.upstream.product.deleteEnvironment(workspaceId, productId, envId);
}

export function getEnvironmentStatus(ctx: BffContext, workspaceId: string, productId: string, envId: string) {
  return ctx.upstream.product.getEnvironmentStatus(workspaceId, productId, envId);
}

export function getSprintProcess(ctx: BffContext, workspaceId: string, productId: string) {
  return ctx.upstream.product.getSprintProcess(workspaceId, productId);
}

export function upsertSprintProcess(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").UpsertSprintProcessCommand,
) {
  return ctx.upstream.product.upsertSprintProcess(workspaceId, productId, input);
}

export async function listSprintProcesses(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listSprintProcesses(workspaceId, productId);
  return data.items;
}

export function createSprintProcess(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateSprintProcessCommand,
) {
  return ctx.upstream.product.createSprintProcess(workspaceId, productId, input);
}

export function updateSprintProcessById(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  processId: string,
  input: import("@sparkops/contracts/product").UpsertSprintProcessCommand,
) {
  return ctx.upstream.product.updateSprintProcessById(workspaceId, productId, processId, input);
}

export function deleteSprintProcess(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  processId: string,
) {
  return ctx.upstream.product.deleteSprintProcess(workspaceId, productId, processId);
}

export async function listWorkspaceSprintProcessTemplates(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listWorkspaceSprintProcessTemplates(workspaceId);
  return data.items;
}

export function createWorkspaceSprintProcessTemplate(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateWorkspaceSprintProcessTemplateCommand,
) {
  return ctx.upstream.product.createWorkspaceSprintProcessTemplate(workspaceId, input);
}

export function updateWorkspaceSprintProcessTemplate(
  ctx: BffContext,
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").UpdateWorkspaceSprintProcessTemplateCommand,
) {
  return ctx.upstream.product.updateWorkspaceSprintProcessTemplate(workspaceId, templateId, input);
}

export function deleteWorkspaceSprintProcessTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.deleteWorkspaceSprintProcessTemplate(workspaceId, templateId);
}

export function importWorkspaceSprintTemplate(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").ImportWorkspaceSprintTemplateCommand,
) {
  return ctx.upstream.product.importWorkspaceSprintTemplate(workspaceId, productId, input);
}

export async function listSprints(ctx: BffContext, workspaceId: string, productId: string, status?: string) {
  const data = await ctx.upstream.product.listSprints(workspaceId, productId, status);
  return data.items;
}

export function createSprint(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateSprintCommand,
) {
  return ctx.upstream.product.createSprint(workspaceId, productId, input);
}

export function getSprint(ctx: BffContext, workspaceId: string, productId: string, sprintId: string) {
  return ctx.upstream.product.getSprint(workspaceId, productId, sprintId);
}

export function getSprintBoard(ctx: BffContext, workspaceId: string, productId: string, sprintId: string) {
  return ctx.upstream.product.getSprintBoard(workspaceId, productId, sprintId);
}

export function startSprint(ctx: BffContext, workspaceId: string, productId: string, sprintId: string) {
  return ctx.upstream.product.startSprint(workspaceId, productId, sprintId);
}

export function completeSprint(ctx: BffContext, workspaceId: string, productId: string, sprintId: string) {
  return ctx.upstream.product.completeSprint(workspaceId, productId, sprintId);
}

export function createWorkItem(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").CreateWorkItemCommand,
) {
  return ctx.upstream.product.createWorkItem(workspaceId, productId, sprintId, input);
}

export function getWorkItem(ctx: BffContext, workspaceId: string, productId: string, workItemId: string) {
  return ctx.upstream.product.getWorkItem(workspaceId, productId, workItemId);
}

export function updateWorkItem(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").UpdateWorkItemCommand,
) {
  return ctx.upstream.product.updateWorkItem(workspaceId, productId, workItemId, input);
}

export function moveWorkItem(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").MoveWorkItemCommand,
) {
  return ctx.upstream.product.moveWorkItem(workspaceId, productId, workItemId, input);
}

export function deleteWorkItem(ctx: BffContext, workspaceId: string, productId: string, workItemId: string) {
  return ctx.upstream.product.deleteWorkItem(workspaceId, productId, workItemId);
}

export function updateSprint(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").UpdateSprintCommand,
) {
  return ctx.upstream.product.updateSprint(workspaceId, productId, sprintId, input);
}

export function executeWorkItem(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workItemId: string,
  input?: import("@sparkops/contracts/product").ExecuteWorkItemCommand,
) {
  return ctx.upstream.product.executeWorkItem(workspaceId, productId, workItemId, input);
}

export async function listWorkItemExecutions(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workItemId: string,
) {
  const data = await ctx.upstream.product.listWorkItemExecutions(workspaceId, productId, workItemId);
  return data.items;
}

export function importJiraSprint(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").ImportJiraSprintCommand,
) {
  return ctx.upstream.product.importJiraSprint(workspaceId, productId, sprintId, input);
}

export function linkJiraIssue(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").LinkJiraIssueCommand,
) {
  return ctx.upstream.product.linkJiraIssue(workspaceId, productId, workItemId, input);
}

export function importPmSprint(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").ImportPmSprintCommand,
) {
  return ctx.upstream.product.importPmSprint(workspaceId, productId, sprintId, input);
}

export function linkPmIssue(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").LinkPmIssueCommand,
) {
  return ctx.upstream.product.linkPmIssue(workspaceId, productId, workItemId, input);
}

export function unlinkPmIssue(ctx: BffContext, workspaceId: string, productId: string, workItemId: string) {
  return ctx.upstream.product.unlinkPmIssue(workspaceId, productId, workItemId);
}

export function getSprintBoardWsConfig(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  sprintId: string,
): import("@sparkops/contracts/product").SprintBoardWsConfig {
  if (!ctx.token) {
    throw new Error("authorization required");
  }
  return {
    websocket_path: `/api/v1/products/${productId}/sprints/${sprintId}/board/ws`,
    access_token: ctx.token,
    workspace_id: workspaceId,
  };
}

export function getReleasePlanCollabWsConfig(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
): import("@sparkops/contracts/product").ReleasePlanCollabWsConfig {
  if (!ctx.token) {
    throw new Error("authorization required");
  }
  return {
    websocket_path: `/api/v1/products/${productId}/release-plans/${planId}/collaboration/ws`,
    access_token: ctx.token,
    workspace_id: workspaceId,
  };
}

export async function listReleasePlanCollabEditors(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  planId: string,
) {
  return ctx.upstream.product.listReleasePlanCollabEditors(workspaceId, productId, planId);
}

export function getSprintMetrics(ctx: BffContext, workspaceId: string, productId: string, sprintId: string) {
  return ctx.upstream.product.getSprintMetrics(workspaceId, productId, sprintId);
}

export async function listActiveSprints(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listActiveSprints(workspaceId);
  return data.items;
}
