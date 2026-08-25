import type { BffContext } from "@sparkops/bff-core";
import type { GitTreeListVM, ListGitTreeCommand } from "@sparkops/contracts/workspace";

export async function listGitTree(
  ctx: BffContext,
  workspaceId: string,
  input: ListGitTreeCommand,
): Promise<GitTreeListVM> {
  return ctx.upstream.workspace.listGitTree(workspaceId, input);
}
