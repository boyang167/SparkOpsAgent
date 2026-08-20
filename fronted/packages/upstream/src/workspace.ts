import type { ProjectListVM, ProjectVM } from "@sparkops/contracts/workspace";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createWorkspaceUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listProjects(workspaceId: string) {
      return upstreamFetch<ProjectListVM>(`/api/v1/workspaces/${workspaceId}/projects`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async getProject(workspaceId: string, projectId: string) {
      return upstreamFetch<ProjectVM>(`/api/v1/projects/${projectId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async createProject(workspaceId: string, input: import("@sparkops/contracts/workspace").CreateProjectCommand) {
      return upstreamFetch<import("@sparkops/contracts/workspace").ProjectVM>(
        `/api/v1/workspaces/${workspaceId}/projects`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateProject(
      workspaceId: string,
      projectId: string,
      input: import("@sparkops/contracts/workspace").UpdateProjectCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/workspace").ProjectVM>(
        `/api/v1/projects/${projectId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async archiveProject(workspaceId: string, projectId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/projects/${projectId}`, {
        method: "DELETE",
        options: { ...options, workspaceId },
      });
    },

    async submitProjectReview(workspaceId: string, projectId: string) {
      return upstreamFetch<import("@sparkops/contracts/workspace").ProjectVM>(
        `/api/v1/projects/${projectId}/submit-review`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async reviewProject(
      workspaceId: string,
      projectId: string,
      input: import("@sparkops/contracts/workspace").DecideProjectReviewCommand,
    ) {
      return upstreamFetch<import("@sparkops/contracts/workspace").ProjectVM>(
        `/api/v1/projects/${projectId}/review`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async listProjectMembers(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/workspace").ProjectMemberVM[] }>(
        `/api/v1/projects/${projectId}/members`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async addProjectMember(
      workspaceId: string,
      projectId: string,
      input: import("@sparkops/contracts/workspace").AddProjectMemberCommand,
    ) {
      return upstreamFetch<{ added: true }>(`/api/v1/projects/${projectId}/members`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async updateProjectMemberRole(
      workspaceId: string,
      projectId: string,
      memberId: string,
      input: import("@sparkops/contracts/workspace").UpdateProjectMemberRoleCommand,
    ) {
      return upstreamFetch<{ updated: true }>(`/api/v1/projects/${projectId}/members/${memberId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async removeProjectMember(workspaceId: string, projectId: string, memberId: string) {
      return upstreamFetch<{ removed: true }>(`/api/v1/projects/${projectId}/members/${memberId}`, {
        method: "DELETE",
        options: { ...options, workspaceId },
      });
    },

    async listGeneratedTemplates(workspaceId: string, projectId: string) {
      return upstreamFetch<import("@sparkops/contracts/workspace").GeneratedTemplateListVM>(
        `/api/v1/projects/${projectId}/generated-templates`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
  };
}

export type WorkspaceUpstream = ReturnType<typeof createWorkspaceUpstream>;
