import type { BffContext } from "@sparkops/bff-core";
import type { GitRefListVM, ListGitRefsCommand } from "@sparkops/contracts/workspace";

export async function listGitRefs(
  ctx: BffContext,
  workspaceId: string,
  input: ListGitRefsCommand,
): Promise<GitRefListVM> {
  return ctx.upstream.workspace.listGitRefs(workspaceId, input);
}
