import type { BffContext } from "@sparkops/bff-core";
import type { GetGitFileContentCommand } from "@sparkops/contracts/workspace";

export async function getGitFileContent(
  ctx: BffContext,
  workspaceId: string,
  input: GetGitFileContentCommand,
) {
  return ctx.upstream.workspace.getGitFileContent(workspaceId, input);
}
