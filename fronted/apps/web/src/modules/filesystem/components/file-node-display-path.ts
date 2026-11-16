import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { getFileNode } from "@sparkops/web-sdk/filesystem";

export function formatNodePath(segments: string[]): string {
  const cleaned = segments.map((segment) => segment.trim()).filter(Boolean);
  if (cleaned.length === 0) {
    return "/";
  }
  return `/${cleaned.join("/")}`;
}

export function nodePathInFolder(folderLabel: string | undefined, node: FileNodeVM): string {
  const segments: string[] = [];
  if (folderLabel && folderLabel !== "根目录") {
    segments.push(folderLabel);
  }
  segments.push(node.name);
  return formatNodePath(segments);
}

export async function resolveNodePath(
  workspaceId: string,
  node: FileNodeVM,
  rootNodeId: string | null | undefined,
  signal?: AbortSignal,
): Promise<string> {
  const segments: string[] = [];
  const seen = new Set<string>();
  let current: FileNodeVM | null = node;

  while (current) {
    if (seen.has(current.id)) {
      break;
    }
    seen.add(current.id);
    if (!rootNodeId || current.id !== rootNodeId) {
      segments.push(current.name);
    }
    const parentId = current.parent_id;
    if (!parentId) {
      break;
    }
    if (rootNodeId && parentId === rootNodeId) {
      break;
    }
    current = await getFileNode(
      workspaceId,
      parentId,
      { namespace_id: node.namespace_id },
      { signal },
    );
  }

  segments.reverse();
  return formatNodePath(segments);
}