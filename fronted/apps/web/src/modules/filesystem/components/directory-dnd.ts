import type { FileNodeVM } from "@sparkops/contracts/filesystem";

export const FILE_NODE_DRAG_TYPE = "application/x-sparkops-file-nodes";

export type DraggedNode = Pick<FileNodeVM, "id" | "type" | "state"> & {
  parentId?: string | null;
};

export type FileNodeDragPayload = {
  workspaceId: string;
  nodes: DraggedNode[];
};

export type DropInput = {
  draggedIds: string[];
  targetId: string;
  descendantIds: ReadonlySet<string>;
  sameWorkspace: boolean;
  canWrite: boolean;
  allActive: boolean;
};

export type DropDecision =
  | { allowed: true }
  | { allowed: false; reason: string };

export function validateNodeDrop(input: DropInput): DropDecision {
  if (!input.canWrite) return { allowed: false, reason: "没有移动权限" };
  if (!input.sameWorkspace) return { allowed: false, reason: "不能跨工作区移动" };
  if (!input.allActive) return { allowed: false, reason: "不能移动不可用的项目" };
  if (input.draggedIds.includes(input.targetId)) {
    return { allowed: false, reason: "不能移动到自身" };
  }
  if (input.descendantIds.has(input.targetId)) {
    return { allowed: false, reason: "不能移动到子目录" };
  }
  return { allowed: true };
}

export function readNodeDragPayload(dataTransfer: DataTransfer): FileNodeDragPayload | null {
  try {
    const value = JSON.parse(dataTransfer.getData(FILE_NODE_DRAG_TYPE)) as FileNodeDragPayload;
    if (
      typeof value.workspaceId !== "string" ||
      !Array.isArray(value.nodes) ||
      value.nodes.some((node) => typeof node.id !== "string")
    ) {
      return null;
    }
    return value;
  } catch {
    return null;
  }
}

export function writeNodeDragPayload(
  dataTransfer: DataTransfer,
  workspaceId: string,
  nodes: FileNodeVM[],
) {
  const payload: FileNodeDragPayload = {
    workspaceId,
    nodes: nodes.map((node) => ({
      id: node.id,
      parentId: node.parent_id,
      type: node.type,
      state: node.state,
    })),
  };
  dataTransfer.effectAllowed = "move";
  dataTransfer.setData(FILE_NODE_DRAG_TYPE, JSON.stringify(payload));
}
