import type {
  ProductOverviewVM,
  ProductPlanVM,
  ProductProjectVM,
  ProductVM,
  ProductWorkflowRunVM,
  ProductWorkflowVM,
  ReleasePlanVM,
  WorkflowTemplatePreviewVM,
  WorkflowTemplateVM,
} from "@sparkops/contracts/product";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createProductUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listProducts(workspaceId: string) {
      return upstreamFetch<{ items: ProductVM[] }>(
        `/api/v1/workspaces/${workspaceId}/products`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createProduct(workspaceId: string, input: import("@sparkops/contracts/product").CreateProductCommand) {
      return upstreamFetch<ProductVM>(
        `/api/v1/workspaces/${workspaceId}/products`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async getProduct(workspaceId: string, productId: string) {
      return upstreamFetch<ProductVM>(
        `/api/v1/products/${productId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getProductOverview(workspaceId: string, productId: string) {
      return upstreamFetch<ProductOverviewVM>(
        `/api/v1/products/${productId}/overview`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listProductProjects(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: ProductProjectVM[] }>(
        `/api/v1/products/${productId}/projects`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listProductPlans(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: ProductPlanVM[] }>(
        `/api/v1/products/${productId}/plans`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listRequirements(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").ProductRequirementVM[] }>(
        `/api/v1/products/${productId}/requirements`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createRequirement(workspaceId: string, productId: string, input: Record<string, unknown>) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/requirements`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async approveRequirement(workspaceId: string, productId: string, reqId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/requirements/${reqId}/approve`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async rejectRequirement(workspaceId: string, productId: string, reqId: string, reason?: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/requirements/${reqId}/reject`,
        { method: "POST", body: JSON.stringify({ reason }), options: { ...options, workspaceId } },
      );
    },

    async releaseRequirement(workspaceId: string, productId: string, reqId: string) {
      return upstreamFetch<{ id: string; status: string; build_id?: string }>(
        `/api/v1/products/${productId}/requirements/${reqId}/release`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async listTemplates(workspaceId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").TemplateVM[] }>(
        `/api/v1/workspaces/${workspaceId}/templates`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getTemplate(workspaceId: string, templateId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").TemplateVM>(
        `/api/v1/workspaces/${workspaceId}/templates/${templateId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createTemplate(workspaceId: string, input: Record<string, unknown>) {
      return upstreamFetch<import("@sparkops/contracts/product").TemplateVM>(
        `/api/v1/workspaces/${workspaceId}/templates`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateTemplate(workspaceId: string, templateId: string, input: Record<string, unknown>) {
      return upstreamFetch<import("@sparkops/contracts/product").TemplateVM>(
        `/api/v1/workspaces/${workspaceId}/templates/${templateId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteTemplate(workspaceId: string, templateId: string) {
      return upstreamFetch<{ deleted: boolean }>(
        `/api/v1/workspaces/${workspaceId}/templates/${templateId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async listWorkflowTemplates(workspaceId: string) {
      return upstreamFetch<{ items: WorkflowTemplateVM[] }>(
        `/api/v1/workspaces/${workspaceId}/workflow-templates`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getWorkflowTemplate(workspaceId: string, templateId: string) {
      return upstreamFetch<WorkflowTemplateVM>(
        `/api/v1/workspaces/${workspaceId}/workflow-templates/${templateId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async previewWorkflowTemplate(workspaceId: string, templateId: string, keyVals: Record<string, string>) {
      return upstreamFetch<WorkflowTemplatePreviewVM>(
        `/api/v1/workspaces/${workspaceId}/workflow-templates/${templateId}/preview`,
        { method: "POST", body: JSON.stringify({ key_vals: keyVals }), options: { ...options, workspaceId } },
      );
    },

    async createWorkflowTemplate(
      workspaceId: string,
      input: import("@sparkops/contracts/product").CreateWorkflowTemplateCommand,
    ) {
      return upstreamFetch<{ id: string; status: string; version: string }>(
        `/api/v1/workspaces/${workspaceId}/workflow-templates`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateWorkflowTemplate(
      workspaceId: string,
      templateId: string,
      input: import("@sparkops/contracts/product").UpdateWorkflowTemplateCommand,
    ) {
      return upstreamFetch<{ id: string; status: string; version: string }>(
        `/api/v1/workspaces/${workspaceId}/workflow-templates/${templateId}`,
        { method: "PUT", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async publishWorkflowTemplate(workspaceId: string, templateId: string) {
      return upstreamFetch<{ id: string; status: string; version: string }>(
        `/api/v1/workspaces/${workspaceId}/workflow-templates/${templateId}/publish`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async listWorkspaceWorkflows(workspaceId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").WorkspaceProductWorkflowVM[] }>(
        `/api/v1/workspaces/${workspaceId}/workflows`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listReleasePlans(workspaceId: string, productId: string, sprintId?: string) {
      const query = sprintId ? `?sprint_id=${encodeURIComponent(sprintId)}` : "";
      return upstreamFetch<{ items: ReleasePlanVM[] }>(
        `/api/v1/products/${productId}/release-plans${query}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getReleasePlan(workspaceId: string, productId: string, planId: string) {
      return upstreamFetch<ReleasePlanVM>(
        `/api/v1/products/${productId}/release-plans/${planId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createReleasePlan(workspaceId: string, productId: string, input: Record<string, unknown>) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async addReleasePlanJob(workspaceId: string, productId: string, planId: string, input: Record<string, unknown>) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans/${planId}/jobs`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async finishReleasePlanPlanning(workspaceId: string, productId: string, planId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans/${planId}/finish-planning`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async approveReleasePlan(workspaceId: string, productId: string, planId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans/${planId}/approve`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async markReleasePlanJobDone(workspaceId: string, productId: string, planId: string, jobId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans/${planId}/jobs/${jobId}/done`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async skipReleasePlanJob(workspaceId: string, productId: string, planId: string, jobId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans/${planId}/jobs/${jobId}/skip`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async executeReleasePlan(workspaceId: string, productId: string, planId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans/${planId}/execute`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async retryReleasePlanJob(workspaceId: string, productId: string, planId: string, jobId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/release-plans/${planId}/jobs/${jobId}/retry`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async listProductWorkflows(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: ProductWorkflowVM[] }>(
        `/api/v1/products/${productId}/workflows`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getProductWorkflow(workspaceId: string, productId: string, workflowId: string) {
      return upstreamFetch<ProductWorkflowVM>(
        `/api/v1/products/${productId}/workflows/${workflowId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createBlankProductWorkflow(
      workspaceId: string,
      productId: string,
      input: import("@sparkops/contracts/product").CreateBlankProductWorkflowCommand,
    ) {
      return upstreamFetch<ProductWorkflowVM>(
        `/api/v1/products/${productId}/workflows`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateProductWorkflow(
      workspaceId: string,
      productId: string,
      workflowId: string,
      input: import("@sparkops/contracts/product").UpdateProductWorkflowCommand,
    ) {
      return upstreamFetch<ProductWorkflowVM>(
        `/api/v1/products/${productId}/workflows/${workflowId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteProductWorkflow(workspaceId: string, productId: string, workflowId: string) {
      return upstreamFetch<{ ok: boolean }>(
        `/api/v1/products/${productId}/workflows/${workflowId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async createProductWorkflowFromTemplate(
      workspaceId: string,
      productId: string,
      input: import("@sparkops/contracts/product").CreateProductWorkflowCommand,
    ) {
      return upstreamFetch<{ id: string; name: string; key: string; status: string }>(
        `/api/v1/products/${productId}/workflows/from-template`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async startProductWorkflowRun(
      workspaceId: string,
      productId: string,
      workflowId: string,
      input?: import("@sparkops/contracts/product").StartProductWorkflowRunCommand,
    ) {
      return upstreamFetch<ProductWorkflowRunVM>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs`,
        { method: "POST", body: JSON.stringify(input ?? {}), options: { ...options, workspaceId } },
      );
    },

    async listProductWorkflowRuns(workspaceId: string, productId: string, workflowId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").ProductWorkflowRunSummaryVM[] }>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getProductWorkflowRun(
      workspaceId: string,
      productId: string,
      workflowId: string,
      runId: string,
    ) {
      return upstreamFetch<ProductWorkflowRunVM>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs/${runId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async updateProductWorkflowRunRemark(
      workspaceId: string,
      productId: string,
      workflowId: string,
      runId: string,
      input: import("@sparkops/contracts/product").UpdateProductWorkflowRunRemarkCommand,
    ) {
      return upstreamFetch<{ ok: boolean }>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs/${runId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async approveWorkflowJob(
      workspaceId: string,
      productId: string,
      workflowId: string,
      runId: string,
      jobName: string,
    ) {
      return upstreamFetch<{ ok: boolean }>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs/${runId}/jobs/${encodeURIComponent(jobName)}/approve`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async rejectWorkflowJob(
      workspaceId: string,
      productId: string,
      workflowId: string,
      runId: string,
      jobName: string,
      input?: import("@sparkops/contracts/product").RejectWorkflowJobCommand,
    ) {
      return upstreamFetch<{ ok: boolean }>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs/${runId}/jobs/${encodeURIComponent(jobName)}/reject`,
        { method: "POST", body: JSON.stringify(input ?? {}), options: { ...options, workspaceId } },
      );
    },

    async continueProductWorkflowRun(workspaceId: string, productId: string, workflowId: string, runId: string) {
      return upstreamFetch<{ ok: boolean }>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs/${runId}/continue`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async pauseProductWorkflowRun(
      workspaceId: string,
      productId: string,
      workflowId: string,
      runId: string,
    ) {
      return upstreamFetch<ProductWorkflowRunVM>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs/${runId}/pause`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async cancelProductWorkflowRun(
      workspaceId: string,
      productId: string,
      workflowId: string,
      runId: string,
      input?: import("@sparkops/contracts/product").CancelProductWorkflowRunCommand,
    ) {
      return upstreamFetch<ProductWorkflowRunVM>(
        `/api/v1/products/${productId}/workflows/${workflowId}/runs/${runId}/cancel`,
        { method: "POST", body: JSON.stringify(input ?? {}), options: { ...options, workspaceId } },
      );
    },

    async listEnvironments(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").EnvironmentVM[] }>(
        `/api/v1/products/${productId}/environments`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createEnvironment(
      workspaceId: string,
      productId: string,
      input: import("@sparkops/contracts/product").CreateEnvironmentCommand,
    ) {
      return upstreamFetch<{ id: string; name: string; status: string }>(
        `/api/v1/products/${productId}/environments`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateEnvironment(
      workspaceId: string,
      productId: string,
      envId: string,
      input: import("@sparkops/contracts/product").UpdateEnvironmentCommand,
    ) {
      return upstreamFetch<{ id: string; name: string; status: string }>(
        `/api/v1/products/${productId}/environments/${envId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteEnvironment(workspaceId: string, productId: string, envId: string) {
      return upstreamFetch<{ deleted: boolean }>(
        `/api/v1/products/${productId}/environments/${envId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async getEnvironmentStatus(workspaceId: string, productId: string, envId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").EnvironmentStatusVM>(
        `/api/v1/products/${productId}/environments/${envId}/status`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getSprintProcess(workspaceId: string, productId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintProcessVM>(
        `/api/v1/products/${productId}/sprint-process`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async upsertSprintProcess(
      workspaceId: string,
      productId: string,
      input: import("@sparkops/contracts/product").UpsertSprintProcessCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintProcessVM>(
        `/api/v1/products/${productId}/sprint-process`,
        { method: "PUT", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async listSprintProcesses(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").SprintProcessVM[] }>(
        `/api/v1/products/${productId}/sprint-processes`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createSprintProcess(
      workspaceId: string,
      productId: string,
      input: import("@sparkops/contracts/product").CreateSprintProcessCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintProcessVM>(
        `/api/v1/products/${productId}/sprint-processes`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateSprintProcessById(
      workspaceId: string,
      productId: string,
      processId: string,
      input: import("@sparkops/contracts/product").UpsertSprintProcessCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintProcessVM>(
        `/api/v1/products/${productId}/sprint-processes/${processId}`,
        { method: "PUT", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteSprintProcess(workspaceId: string, productId: string, processId: string) {
      return upstreamFetch<void>(
        `/api/v1/products/${productId}/sprint-processes/${processId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async listWorkspaceSprintProcessTemplates(workspaceId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").WorkspaceSprintProcessTemplateVM[] }>(
        `/api/v1/workspaces/${workspaceId}/sprint-process-templates`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createWorkspaceSprintProcessTemplate(
      workspaceId: string,
      input: import("@sparkops/contracts/product").CreateWorkspaceSprintProcessTemplateCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/product").WorkspaceSprintProcessTemplateVM>(
        `/api/v1/workspaces/${workspaceId}/sprint-process-templates`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteWorkspaceSprintProcessTemplate(workspaceId: string, templateId: string) {
      return upstreamFetch<void>(
        `/api/v1/workspaces/${workspaceId}/sprint-process-templates/${templateId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async importWorkspaceSprintTemplate(
      workspaceId: string,
      productId: string,
      input: import("@sparkops/contracts/product").ImportWorkspaceSprintTemplateCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintProcessVM>(
        `/api/v1/workspaces/${workspaceId}/products/${productId}/sprint-processes/import-workspace`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async listSprints(workspaceId: string, productId: string, status?: string) {
      const qs = status ? `?status=${encodeURIComponent(status)}` : "";
      return upstreamFetch<{ items: import("@sparkops/contracts/product").SprintSummaryVM[] }>(
        `/api/v1/products/${productId}/sprints${qs}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createSprint(
      workspaceId: string,
      productId: string,
      input: import("@sparkops/contracts/product").CreateSprintCommand,
    ) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/sprints`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async getSprint(workspaceId: string, productId: string, sprintId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintVM>(
        `/api/v1/products/${productId}/sprints/${sprintId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getSprintBoard(workspaceId: string, productId: string, sprintId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintBoardVM>(
        `/api/v1/products/${productId}/sprints/${sprintId}/board`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async startSprint(workspaceId: string, productId: string, sprintId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/sprints/${sprintId}/start`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async completeSprint(workspaceId: string, productId: string, sprintId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/sprints/${sprintId}/complete`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async createWorkItem(
      workspaceId: string,
      productId: string,
      sprintId: string,
      input: import("@sparkops/contracts/product").CreateWorkItemCommand,
    ) {
      return upstreamFetch<{ id: string }>(
        `/api/v1/products/${productId}/sprints/${sprintId}/work-items`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async getWorkItem(workspaceId: string, productId: string, workItemId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").WorkItemVM>(
        `/api/v1/products/${productId}/work-items/${workItemId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async updateWorkItem(
      workspaceId: string,
      productId: string,
      workItemId: string,
      input: import("@sparkops/contracts/product").UpdateWorkItemCommand,
    ) {
      return upstreamFetch<{ id: string; version: number }>(
        `/api/v1/products/${productId}/work-items/${workItemId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async moveWorkItem(
      workspaceId: string,
      productId: string,
      workItemId: string,
      input: import("@sparkops/contracts/product").MoveWorkItemCommand,
    ) {
      return upstreamFetch<{ id: string; stage_id: string; version: number }>(
        `/api/v1/products/${productId}/work-items/${workItemId}/move`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteWorkItem(workspaceId: string, productId: string, workItemId: string) {
      return upstreamFetch<void>(
        `/api/v1/products/${productId}/work-items/${workItemId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async updateSprint(
      workspaceId: string,
      productId: string,
      sprintId: string,
      input: import("@sparkops/contracts/product").UpdateSprintCommand,
    ) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/sprints/${sprintId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async executeWorkItem(
      workspaceId: string,
      productId: string,
      workItemId: string,
      input?: import("@sparkops/contracts/product").ExecuteWorkItemCommand,
    ) {
      return upstreamFetch<{ execution_id: string; workflow_run_id: string }>(
        `/api/v1/products/${productId}/work-items/${workItemId}/execute`,
        { method: "POST", body: JSON.stringify(input ?? {}), options: { ...options, workspaceId } },
      );
    },

    async listWorkItemExecutions(workspaceId: string, productId: string, workItemId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").WorkItemExecutionVM[] }>(
        `/api/v1/products/${productId}/work-items/${workItemId}/executions`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async importJiraSprint(
      workspaceId: string,
      productId: string,
      sprintId: string,
      input: import("@sparkops/contracts/product").ImportJiraSprintCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/product").ImportJiraSprintResult>(
        `/api/v1/products/${productId}/sprints/${sprintId}/import/jira`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async linkJiraIssue(
      workspaceId: string,
      productId: string,
      workItemId: string,
      input: import("@sparkops/contracts/product").LinkJiraIssueCommand,
    ) {
      return upstreamFetch<{ id: string; external?: import("@sparkops/contracts/product").WorkItemExternalRef }>(
        `/api/v1/products/${productId}/work-items/${workItemId}/link-jira`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async importPmSprint(
      workspaceId: string,
      productId: string,
      sprintId: string,
      input: import("@sparkops/contracts/product").ImportPmSprintCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/product").ImportPmSprintResult>(
        `/api/v1/products/${productId}/sprints/${sprintId}/import/pm`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async linkPmIssue(
      workspaceId: string,
      productId: string,
      workItemId: string,
      input: import("@sparkops/contracts/product").LinkPmIssueCommand,
    ) {
      return upstreamFetch<{ id: string; external?: import("@sparkops/contracts/product").WorkItemExternalRef }>(
        `/api/v1/products/${productId}/work-items/${workItemId}/link-pm`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async getSprintMetrics(workspaceId: string, productId: string, sprintId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").SprintMetricsVM>(
        `/api/v1/products/${productId}/sprints/${sprintId}/metrics`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listActiveSprints(workspaceId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").ActiveSprintSummaryVM[] }>(
        `/api/v1/workspaces/${workspaceId}/sprints/active`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
  };
}

export type ProductUpstream = ReturnType<typeof createProductUpstream>;
