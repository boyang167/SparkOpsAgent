import type { FileNodeVM } from "@sparkops/contracts/filesystem";

export type FileExplorerDialog =
  | { type: "create" }
  | { type: "rename"; node: FileNodeVM }
  | null;

export function showCreateDialog(): FileExplorerDialog {
  return { type: "create" };
}

export function showRenameDialog(node: FileNodeVM): FileExplorerDialog {
  return { type: "rename", node };
}

export function createFileBrowserMoveAdapter(onMove?: (nodeId: string) => void) {
  return {
    canMove: Boolean(onMove),
    onMove: (node: FileNodeVM) => onMove?.(node.id),
  };
}
