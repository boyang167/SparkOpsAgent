import type { FileNodeVM } from "@sparkops/contracts/filesystem";

export type FileExplorerDialog =
  | { type: "create" }
  | { type: "rename"; node: FileNodeVM }
  | { type: "delete"; nodes: FileNodeVM[] }
  | null;

export function showCreateDialog(): FileExplorerDialog {
  return { type: "create" };
}

export function showRenameDialog(node: FileNodeVM): FileExplorerDialog {
  return { type: "rename", node };
}

export function showDeleteDialog(nodes: FileNodeVM[]): FileExplorerDialog {
  return { type: "delete", nodes };
}

export function createFileBrowserMoveAdapter(onMove?: (nodeId: string) => void) {
  return {
    canMove: Boolean(onMove),
    onMove: (node: FileNodeVM) => onMove?.(node.id),
  };
}
