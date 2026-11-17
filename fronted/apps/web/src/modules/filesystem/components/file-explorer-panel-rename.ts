import { renameFileNode } from "@sparkops/web-sdk/filesystem";
import { renameNodeKeepingIdentity } from "./file-explorer-rename";

export type PanelRenameHandler = (nodeId: string, name: string) => Promise<void>;

export function createPanelRenameHandler(
  workspaceId: string,
  namespaceId: string | undefined,
  onRename?: PanelRenameHandler,
): PanelRenameHandler {
  if (onRename) {
    return onRename;
  }
  return async (nodeId, name) => {
    await renameFileNode(workspaceId, nodeId, { name }, { namespaceId });
  };
}

export async function submitPanelRename(
  nodeId: string,
  nextName: string,
  rename: PanelRenameHandler,
): Promise<string> {
  const trimmed = nextName.trim();
  if (!trimmed) {
    throw new Error("name required");
  }
  await renameNodeKeepingIdentity(nodeId, trimmed, rename);
  return trimmed;
}
