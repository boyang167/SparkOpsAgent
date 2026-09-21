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

    async listReleasePlans(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: ReleasePlanVM[] }>(
        `/api/v1/products/${productId}/release-plans`,
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
  };
}

export type ProductUpstream = ReturnType<typeof createProductUpstream>;
