import type {
  CreateProjectCommand,
  DecideProjectReviewCommand,
} from "@sparkops/contracts/workspace";
import { bffFetch, type BffFetchOptions } from "./client";

function withWorkspace(workspaceId: string, options: BffFetchOptions = {}): BffFetchOptions {
  return { ...options, workspaceId };
}

export async function listProjects(workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/workspace").ProjectVM[] }>(
    `/workspaces/${workspaceId}/projects`,
    withWorkspace(workspaceId),
  );
  return data.items;
}

export function getProject(workspaceId: string, projectId: string) {
  return bffFetch<import("@sparkops/contracts/workspace").ProjectVM>(
    `/workspaces/${workspaceId}/projects/${projectId}`,
    withWorkspace(workspaceId),
  );
}

export function createProject(workspaceId: string, input: CreateProjectCommand) {
  return bffFetch<import("@sparkops/contracts/workspace").ProjectVM>(`/workspaces/${workspaceId}/projects`, {
    method: "POST",
    body: JSON.stringify(input),
    ...withWorkspace(workspaceId),
  });
}

export function updateProject(
  workspaceId: string,
  projectId: string,
  input: CreateProjectCommand,
) {
  return bffFetch<import("@sparkops/contracts/workspace").ProjectVM>(
    `/workspaces/${workspaceId}/projects/${projectId}`,
    {
      method: "PATCH",
      body: JSON.stringify(input),
      ...withWorkspace(workspaceId),
    },
  );
}

export function archiveProject(workspaceId: string, projectId: string) {
  return bffFetch<{ deleted: true }>(`/workspaces/${workspaceId}/projects/${projectId}`, {
    method: "DELETE",
    ...withWorkspace(workspaceId),
  });
}

export function submitProjectReview(workspaceId: string, projectId: string) {
  return bffFetch<import("@sparkops/contracts/workspace").ProjectVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/submit-review`,
    {
      method: "POST",
      ...withWorkspace(workspaceId),
    },
  );
}

export function reviewProject(
  workspaceId: string,
  projectId: string,
  input: DecideProjectReviewCommand,
) {
  return bffFetch<import("@sparkops/contracts/workspace").ProjectVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/review`,
    {
      method: "POST",
      body: JSON.stringify(input),
      ...withWorkspace(workspaceId),
    },
  );
}

export function listProjectMembers(workspaceId: string, projectId: string) {
  return bffFetch<{ items: import("@sparkops/contracts/workspace").ProjectMemberVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/members`,
    withWorkspace(workspaceId),
  ).then((d) => d.items);
}

export function addProjectMember(
  workspaceId: string,
  projectId: string,
  input: { member_id: string; role?: "lead" | "member" | "viewer" },
) {
  return bffFetch<{ added: true }>(`/workspaces/${workspaceId}/projects/${projectId}/members`, {
    method: "POST",
    body: JSON.stringify(input),
    ...withWorkspace(workspaceId),
  });
}

export function updateProjectMemberRole(
  workspaceId: string,
  projectId: string,
  memberId: string,
  role: "lead" | "member" | "viewer",
) {
  return bffFetch<{ updated: true }>(
    `/workspaces/${workspaceId}/projects/${projectId}/members/${memberId}`,
    {
      method: "PATCH",
      body: JSON.stringify({ role }),
      ...withWorkspace(workspaceId),
    },
  );
}

export function removeProjectMember(workspaceId: string, projectId: string, memberId: string) {
  return bffFetch<{ removed: true }>(
    `/workspaces/${workspaceId}/projects/${projectId}/members/${memberId}`,
    {
      method: "DELETE",
      ...withWorkspace(workspaceId),
    },
  );
}

export async function listGeneratedTemplates(workspaceId: string, projectId: string) {
  const data = await bffFetch<import("@sparkops/contracts/workspace").GeneratedTemplateListVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/generated-templates`,
    withWorkspace(workspaceId),
  );
  return data.items;
}

export function updateGeneratedTemplate(
  workspaceId: string,
  projectId: string,
  kind: string,
  input: import("@sparkops/contracts/workspace").UpdateGeneratedTemplateCommand,
) {
  return bffFetch<import("@sparkops/contracts/workspace").GeneratedTemplateArtifact>(
    `/workspaces/${workspaceId}/projects/${projectId}/generated-templates/${encodeURIComponent(kind)}`,
    {
      method: "PUT",
      body: JSON.stringify(input),
      ...withWorkspace(workspaceId),
    },
  );
}

export async function listProjectVersions(workspaceId: string, projectId: string) {
  const data = await bffFetch<import("@sparkops/contracts/workspace").ProjectVersionListVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/versions`,
    withWorkspace(workspaceId),
  );
  return data.items;
}

export function getProjectVersion(workspaceId: string, projectId: string, versionId: string) {
  return bffFetch<import("@sparkops/contracts/workspace").ProjectVersionDetailVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/versions/${versionId}`,
    withWorkspace(workspaceId),
  );
}

export async function listGitRefs(
  workspaceId: string,
  input: import("@sparkops/contracts/workspace").ListGitRefsCommand,
) {
  const data = await bffFetch<import("@sparkops/contracts/workspace").GitRefListVM>(
    `/workspaces/${workspaceId}/git/refs`,
    {
      method: "POST",
      body: JSON.stringify(input),
      ...withWorkspace(workspaceId),
    },
  );
  return data.items;
}

export async function listGitTree(
  workspaceId: string,
  input: import("@sparkops/contracts/workspace").ListGitTreeCommand,
) {
  const data = await bffFetch<import("@sparkops/contracts/workspace").GitTreeListVM>(
    `/workspaces/${workspaceId}/git/tree`,
    {
      method: "POST",
      body: JSON.stringify(input),
      ...withWorkspace(workspaceId),
    },
  );
  return data.items;
}

export async function getGitFileContent(
  workspaceId: string,
  input: import("@sparkops/contracts/workspace").GetGitFileContentCommand,
) {
  return bffFetch<import("@sparkops/contracts/workspace").GitFileContentVM>(
    `/workspaces/${workspaceId}/git/file`,
    {
      method: "POST",
      body: JSON.stringify(input),
      ...withWorkspace(workspaceId),
    },
  );
}
