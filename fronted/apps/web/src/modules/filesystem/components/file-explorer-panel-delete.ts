import { trashFileNode } from "@sparkops/web-sdk/filesystem";

type TrashTarget = {
  name: string;
  type: "FILE" | "DIRECTORY";
};

export type TrashNodesResult = {
  ok: boolean;
  removed: string[];
  message: string | null;
};

export function deleteDialogCopy(nodes: TrashTarget[]) {
  if (nodes.length === 1) {
    const node = nodes[0];
    const kind = node.type === "DIRECTORY" ? "目录" : "文件";
    return {
      title: "移入回收站",
      description: `将${kind}「${node.name}」移入回收站。之后可从回收站恢复。`,
    };
  }
  return {
    title: "移入回收站",
    description: `将所选 ${nodes.length} 项移入回收站。之后可从回收站恢复。`,
  };
}

export async function trashPanelNodes(
  workspaceId: string,
  nodeIds: string[],
  options: {
    namespaceId?: string;
    trash?: (nodeId: string) => Promise<void>;
  } = {},
): Promise<TrashNodesResult> {
  const trash = options.trash ?? ((nodeId: string) =>
    trashFileNode(workspaceId, nodeId, { namespaceId: options.namespaceId }));
  const removed: string[] = [];
  try {
    for (const nodeId of nodeIds) {
      await trash(nodeId);
      removed.push(nodeId);
    }
    return { ok: true, removed, message: null };
  } catch (err) {
    return {
      ok: false,
      removed,
      message: err instanceof Error && err.message ? err.message : "删除失败",
    };
  }
}
