import { bffFetch } from "./client";
import type {
  ProductOverviewVM,
  ProductPlanVM,
  ProductProjectVM,
  ProductVM,
} from "@sparkops/contracts/product";

export async function listProducts(workspaceId: string) {
  const data = await bffFetch<{ items: ProductVM[] }>(`/workspaces/${workspaceId}/products`, {
    workspaceId,
  });
  return data.items;
}

export function createProduct(
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateProductCommand,
) {
  return bffFetch<ProductVM>(`/workspaces/${workspaceId}/products`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function getProduct(workspaceId: string, productId: string) {
  return bffFetch<ProductVM>(`/workspaces/${workspaceId}/products/${productId}`, { workspaceId });
}

export function getProductOverview(workspaceId: string, productId: string) {
  return bffFetch<ProductOverviewVM>(
    `/workspaces/${workspaceId}/products/${productId}/overview`,
    { workspaceId },
  );
}

export async function listProductProjects(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: ProductProjectVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/projects`,
    { workspaceId },
  );
  return data.items;
}

export async function listProductPlans(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: ProductPlanVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/plans`,
    { workspaceId },
  );
  return data.items;
}

export async function listRequirements(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").ProductRequirementVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements`,
    { workspaceId },
  );
  return data.items;
}

export function approveRequirement(workspaceId: string, productId: string, reqId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements/${reqId}/approve`,
    { method: "POST", workspaceId },
  );
}

export function rejectRequirement(
  workspaceId: string,
  productId: string,
  reqId: string,
  reason?: string,
) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements/${reqId}/reject`,
    { method: "POST", body: JSON.stringify({ reason }), workspaceId },
  );
}

export function releaseRequirement(workspaceId: string, productId: string, reqId: string) {
  return bffFetch<{ id: string; status: string; build_id?: string }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements/${reqId}/release`,
    { method: "POST", workspaceId },
  );
}

export async function listTemplates(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").TemplateVM[] }>(
    `/workspaces/${workspaceId}/templates`,
    { workspaceId },
  );
  return data.items;
}

export function getTemplate(workspaceId: string, templateId: string) {
  return bffFetch<import("@sparkops/contracts/product").TemplateVM>(
    `/workspaces/${workspaceId}/templates/${templateId}`,
    { workspaceId },
  );
}

export function createTemplate(
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateTemplateCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").TemplateVM>(
    `/workspaces/${workspaceId}/templates`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function updateTemplate(
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").UpdateTemplateCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").TemplateVM>(
    `/workspaces/${workspaceId}/templates/${templateId}`,
    { method: "PATCH", body: JSON.stringify(input), workspaceId },
  );
}

export function deleteTemplate(workspaceId: string, templateId: string) {
  return bffFetch<{ deleted: boolean }>(
    `/workspaces/${workspaceId}/templates/${templateId}`,
    { method: "DELETE", workspaceId },
  );
}

export async function listWorkflowTemplates(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").WorkflowTemplateVM[] }>(
    `/workspaces/${workspaceId}/workflow-templates`,
    { workspaceId },
  );
  return data.items;
}

export function getWorkflowTemplate(workspaceId: string, templateId: string) {
  return bffFetch<import("@sparkops/contracts/product").WorkflowTemplateVM>(
    `/workspaces/${workspaceId}/workflow-templates/${templateId}`,
    { workspaceId },
  );
}

export function previewWorkflowTemplate(
  workspaceId: string,
  templateId: string,
  keyVals: Record<string, string>,
) {
  return bffFetch<import("@sparkops/contracts/product").WorkflowTemplatePreviewVM>(
    `/workspaces/${workspaceId}/workflow-templates/${templateId}/preview`,
    { method: "POST", body: JSON.stringify({ key_vals: keyVals }), workspaceId },
  );
}

export function createWorkflowTemplate(
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateWorkflowTemplateCommand,
) {
  return bffFetch<{ id: string; status: string; version: string }>(
    `/workspaces/${workspaceId}/workflow-templates`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function updateWorkflowTemplate(
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").UpdateWorkflowTemplateCommand,
) {
  return bffFetch<{ id: string; status: string; version: string }>(
    `/workspaces/${workspaceId}/workflow-templates/${templateId}`,
    { method: "PUT", body: JSON.stringify(input), workspaceId },
  );
}

export function publishWorkflowTemplate(workspaceId: string, templateId: string) {
  return bffFetch<{ id: string; status: string; version: string }>(
    `/workspaces/${workspaceId}/workflow-templates/${templateId}/publish`,
    { method: "POST", workspaceId },
  );
}

export async function listWorkspaceWorkflows(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").WorkspaceProductWorkflowVM[] }>(
    `/workspaces/${workspaceId}/workflows`,
    { workspaceId },
  );
  return data.items;
}

export async function listReleasePlans(workspaceId: string, productId: string, sprintId?: string) {
  const query = sprintId ? `?sprint_id=${encodeURIComponent(sprintId)}` : "";
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").ReleasePlanVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans${query}`,
    { workspaceId },
  );
  return data.items;
}

export function getReleasePlan(workspaceId: string, productId: string, planId: string) {
  return bffFetch<import("@sparkops/contracts/product").ReleasePlanVM>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}`,
    { workspaceId },
  );
}

export function createReleasePlan(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateReleasePlanCommand,
) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function addReleasePlanJob(
  workspaceId: string,
  productId: string,
  planId: string,
  input: import("@sparkops/contracts/product").AddReleasePlanJobCommand,
) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}/jobs`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function finishReleasePlanPlanning(workspaceId: string, productId: string, planId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}/finish-planning`,
    { method: "POST", workspaceId },
  );
}

export function approveReleasePlan(workspaceId: string, productId: string, planId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}/approve`,
    { method: "POST", workspaceId },
  );
}

export function markReleasePlanJobDone(
  workspaceId: string,
  productId: string,
  planId: string,
  jobId: string,
) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}/jobs/${jobId}/done`,
    { method: "POST", workspaceId },
  );
}

export function skipReleasePlanJob(workspaceId: string, productId: string, planId: string, jobId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}/jobs/${jobId}/skip`,
    { method: "POST", workspaceId },
  );
}

export function executeReleasePlan(workspaceId: string, productId: string, planId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}/execute`,
    { method: "POST", workspaceId },
  );
}

export function retryReleasePlanJob(workspaceId: string, productId: string, planId: string, jobId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/release-plans/${planId}/jobs/${jobId}/retry`,
    { method: "POST", workspaceId },
  );
}

export async function listProductWorkflows(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").ProductWorkflowVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows`,
    { workspaceId },
  );
  return data.items;
}

export function getProductWorkflow(workspaceId: string, productId: string, workflowId: string) {
  return bffFetch<import("@sparkops/contracts/product").ProductWorkflowVM>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}`,
    { workspaceId },
  );
}

export function createBlankProductWorkflow(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateBlankProductWorkflowCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").ProductWorkflowVM>(
    `/workspaces/${workspaceId}/products/${productId}/workflows`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function updateProductWorkflow(
  workspaceId: string,
  productId: string,
  workflowId: string,
  input: import("@sparkops/contracts/product").UpdateProductWorkflowCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").ProductWorkflowVM>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}`,
    { method: "PATCH", body: JSON.stringify(input), workspaceId },
  );
}

export function deleteProductWorkflow(workspaceId: string, productId: string, workflowId: string) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}`,
    { method: "DELETE", workspaceId },
  );
}

export function createProductWorkflowFromTemplate(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateProductWorkflowCommand,
) {
  return bffFetch<{ id: string; name: string; key: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/from-template`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function startProductWorkflowRun(
  workspaceId: string,
  productId: string,
  workflowId: string,
  input?: import("@sparkops/contracts/product").StartProductWorkflowRunCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").ProductWorkflowRunVM>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs`,
    { method: "POST", body: JSON.stringify(input ?? {}), workspaceId },
  );
}

export async function listProductWorkflowRuns(workspaceId: string, productId: string, workflowId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").ProductWorkflowRunSummaryVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs`,
    { workspaceId },
  );
  return data.items;
}

export function getProductWorkflowRun(
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
) {
  return bffFetch<import("@sparkops/contracts/product").ProductWorkflowRunVM>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs/${runId}`,
    { workspaceId },
  );
}

export function updateProductWorkflowRunRemark(
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  input: import("@sparkops/contracts/product").UpdateProductWorkflowRunRemarkCommand,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs/${runId}`,
    { method: "PATCH", body: JSON.stringify(input), workspaceId },
  );
}

export function approveWorkflowJob(
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  jobName: string,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs/${runId}/jobs/${encodeURIComponent(jobName)}/approve`,
    { method: "POST", workspaceId },
  );
}

export function rejectWorkflowJob(
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  jobName: string,
  input?: import("@sparkops/contracts/product").RejectWorkflowJobCommand,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs/${runId}/jobs/${encodeURIComponent(jobName)}/reject`,
    { method: "POST", body: JSON.stringify(input ?? {}), workspaceId },
  );
}

export function continueProductWorkflowRun(
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs/${runId}/continue`,
    { method: "POST", workspaceId },
  );
}

export function pauseProductWorkflowRun(
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
) {
  return bffFetch<import("@sparkops/contracts/product").ProductWorkflowRunVM>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs/${runId}/pause`,
    { method: "POST", workspaceId },
  );
}

export function cancelProductWorkflowRun(
  workspaceId: string,
  productId: string,
  workflowId: string,
  runId: string,
  input?: import("@sparkops/contracts/product").CancelProductWorkflowRunCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").ProductWorkflowRunVM>(
    `/workspaces/${workspaceId}/products/${productId}/workflows/${workflowId}/runs/${runId}/cancel`,
    { method: "POST", body: JSON.stringify(input ?? {}), workspaceId },
  );
}

export async function listEnvironments(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").EnvironmentVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/environments`,
    { workspaceId },
  );
  return data.items;
}

export function createEnvironment(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateEnvironmentCommand,
) {
  return bffFetch<{ id: string; name: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/environments`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function updateEnvironment(
  workspaceId: string,
  productId: string,
  envId: string,
  input: import("@sparkops/contracts/product").UpdateEnvironmentCommand,
) {
  return bffFetch<{ id: string; name: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/environments/${envId}`,
    { method: "PATCH", body: JSON.stringify(input), workspaceId },
  );
}

export function deleteEnvironment(workspaceId: string, productId: string, envId: string) {
  return bffFetch<{ deleted: boolean }>(
    `/workspaces/${workspaceId}/products/${productId}/environments/${envId}`,
    { method: "DELETE", workspaceId },
  );
}

export function getEnvironmentStatus(workspaceId: string, productId: string, envId: string) {
  return bffFetch<import("@sparkops/contracts/product").EnvironmentStatusVM>(
    `/workspaces/${workspaceId}/products/${productId}/environments/${envId}/status`,
    { workspaceId },
  );
}

export function getSprintProcess(workspaceId: string, productId: string) {
  return bffFetch<import("@sparkops/contracts/product").SprintProcessVM>(
    `/workspaces/${workspaceId}/products/${productId}/sprint-process`,
    { workspaceId },
  );
}

export function upsertSprintProcess(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").UpsertSprintProcessCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").SprintProcessVM>(
    `/workspaces/${workspaceId}/products/${productId}/sprint-process`,
    { method: "PUT", body: JSON.stringify(input), workspaceId },
  );
}

export async function listSprintProcesses(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").SprintProcessVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/sprint-processes`,
    { workspaceId },
  );
  return data.items;
}

export function createSprintProcess(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateSprintProcessCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").SprintProcessVM>(
    `/workspaces/${workspaceId}/products/${productId}/sprint-processes`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function updateSprintProcessById(
  workspaceId: string,
  productId: string,
  processId: string,
  input: import("@sparkops/contracts/product").UpsertSprintProcessCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").SprintProcessVM>(
    `/workspaces/${workspaceId}/products/${productId}/sprint-processes/${processId}`,
    { method: "PUT", body: JSON.stringify(input), workspaceId },
  );
}

export function deleteSprintProcess(workspaceId: string, productId: string, processId: string) {
  return bffFetch<void>(
    `/workspaces/${workspaceId}/products/${productId}/sprint-processes/${processId}`,
    { method: "DELETE", workspaceId },
  );
}

export async function listWorkspaceSprintProcessTemplates(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").WorkspaceSprintProcessTemplateVM[] }>(
    `/workspaces/${workspaceId}/sprint-process-templates`,
    { workspaceId },
  );
  return data.items;
}

export function createWorkspaceSprintProcessTemplate(
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateWorkspaceSprintProcessTemplateCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").WorkspaceSprintProcessTemplateVM>(
    `/workspaces/${workspaceId}/sprint-process-templates`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function updateWorkspaceSprintProcessTemplate(
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").UpdateWorkspaceSprintProcessTemplateCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").WorkspaceSprintProcessTemplateVM>(
    `/workspaces/${workspaceId}/sprint-process-templates/${templateId}`,
    { method: "PUT", body: JSON.stringify(input), workspaceId },
  );
}

export function deleteWorkspaceSprintProcessTemplate(workspaceId: string, templateId: string) {
  return bffFetch<void>(`/workspaces/${workspaceId}/sprint-process-templates/${templateId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export function importWorkspaceSprintTemplate(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").ImportWorkspaceSprintTemplateCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").SprintProcessVM>(
    `/workspaces/${workspaceId}/products/${productId}/sprint-processes/import-workspace`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export async function listSprints(workspaceId: string, productId: string, status?: string) {
  const qs = status ? `?status=${encodeURIComponent(status)}` : "";
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").SprintSummaryVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/sprints${qs}`,
    { workspaceId },
  );
  return data.items;
}

export function createSprint(
  workspaceId: string,
  productId: string,
  input: import("@sparkops/contracts/product").CreateSprintCommand,
) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/sprints`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function getSprintBoard(workspaceId: string, productId: string, sprintId: string) {
  return bffFetch<import("@sparkops/contracts/product").SprintBoardVM>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/board`,
    { workspaceId },
  );
}

export function startSprint(workspaceId: string, productId: string, sprintId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/start`,
    { method: "POST", workspaceId },
  );
}

export function completeSprint(workspaceId: string, productId: string, sprintId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/complete`,
    { method: "POST", workspaceId },
  );
}

export function createWorkItem(
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").CreateWorkItemCommand,
) {
  return bffFetch<{ id: string }>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/work-items`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function getWorkItem(workspaceId: string, productId: string, workItemId: string) {
  return bffFetch<import("@sparkops/contracts/product").WorkItemVM>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}`,
    { workspaceId },
  );
}

export function updateWorkItem(
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").UpdateWorkItemCommand,
) {
  return bffFetch<{ id: string; version: number }>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}`,
    { method: "PATCH", body: JSON.stringify(input), workspaceId },
  );
}

export function moveWorkItem(
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").MoveWorkItemCommand,
) {
  return bffFetch<{ id: string; stage_id: string; version: number }>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}/move`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function deleteWorkItem(workspaceId: string, productId: string, workItemId: string) {
  return bffFetch<void>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}`,
    { method: "DELETE", workspaceId },
  );
}

export function updateSprint(
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").UpdateSprintCommand,
) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}`,
    { method: "PATCH", body: JSON.stringify(input), workspaceId },
  );
}

export function executeWorkItem(
  workspaceId: string,
  productId: string,
  workItemId: string,
  input?: import("@sparkops/contracts/product").ExecuteWorkItemCommand,
) {
  return bffFetch<{ execution_id: string; workflow_run_id: string }>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}/execute`,
    { method: "POST", body: JSON.stringify(input ?? {}), workspaceId },
  );
}

export async function listWorkItemExecutions(workspaceId: string, productId: string, workItemId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").WorkItemExecutionVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}/executions`,
    { workspaceId },
  );
  return data.items;
}

export function importJiraSprint(
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").ImportJiraSprintCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").ImportJiraSprintResult>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/import/jira`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function linkJiraIssue(
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").LinkJiraIssueCommand,
) {
  return bffFetch<{ id: string }>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}/link-jira`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function importPmSprint(
  workspaceId: string,
  productId: string,
  sprintId: string,
  input: import("@sparkops/contracts/product").ImportPmSprintCommand,
) {
  return bffFetch<import("@sparkops/contracts/product").ImportPmSprintResult>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/import/pm`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function linkPmIssue(
  workspaceId: string,
  productId: string,
  workItemId: string,
  input: import("@sparkops/contracts/product").LinkPmIssueCommand,
) {
  return bffFetch<{ id: string }>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}/link-pm`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function unlinkPmIssue(workspaceId: string, productId: string, workItemId: string) {
  return bffFetch<{ id: string }>(
    `/workspaces/${workspaceId}/products/${productId}/work-items/${workItemId}/link-pm`,
    { method: "DELETE", workspaceId },
  );
}

export function getSprintBoardWsConfig(workspaceId: string, productId: string, sprintId: string) {
  return bffFetch<import("@sparkops/contracts/product").SprintBoardWsConfig>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/board/ws-config`,
    { workspaceId },
  );
}

export function getSprintMetrics(workspaceId: string, productId: string, sprintId: string) {
  return bffFetch<import("@sparkops/contracts/product").SprintMetricsVM>(
    `/workspaces/${workspaceId}/products/${productId}/sprints/${sprintId}/metrics`,
    { workspaceId },
  );
}

export async function listActiveSprints(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").ActiveSprintSummaryVM[] }>(
    `/workspaces/${workspaceId}/sprints/active`,
    { workspaceId },
  );
  return data.items;
}
