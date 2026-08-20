import type { BffContext } from "@sparkops/bff-core";
import type { DecideProjectReviewCommand, ProjectVM } from "@sparkops/contracts/workspace";

export function submitProjectReview(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<ProjectVM> {
  return ctx.upstream.workspace.submitProjectReview(workspaceId, projectId);
}

export function reviewProject(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: DecideProjectReviewCommand,
): Promise<ProjectVM> {
  return ctx.upstream.workspace.reviewProject(workspaceId, projectId, input);
}
