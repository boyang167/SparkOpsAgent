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

export async function listReleasePlans(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listReleasePlans(workspaceId, productId);
  return data.items;
}

export async function getReleasePlan(ctx: BffContext, workspaceId: string, productId: string, planId: string) {
  return ctx.upstream.product.getReleasePlan(workspaceId, productId, planId);
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
