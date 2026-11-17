import { BffError } from "@sparkops/contracts";

export type FileMutationBatchResult = {
  succeeded: string[];
  failures: Array<{ nodeId: string; message: string }>;
};

export function trashMutationMessage(error: unknown): string {
  if (error instanceof BffError) {
    if (error.code === "FILESYSTEM_RESTORE_NAME_CONFLICT") {
      return "原目录已有同名项目";
    }
    if (error.code === "FILESYSTEM_RESTORE_PARENT_UNAVAILABLE") {
      return "原目录不可用，无法恢复";
    }
    if (error.code === "FILESYSTEM_RETENTION_HOLD") {
      return "项目受保留策略保护";
    }
    if (error.code === "FILESYSTEM_INVALID_REQUEST") {
      return "请求无效，请刷新页面后重试";
    }
    if (error.code === "FILESYSTEM_CONFLICT") {
      return "项目状态已变更，请刷新回收站后重试";
    }
    return error.message || "操作失败";
  }
  return error instanceof Error && error.message ? error.message : "操作失败";
}

/** Sequentially applies a trash mutation and reports partial success. */
export async function runTrashBatch(
  nodeIds: string[],
  mutate: (nodeId: string) => Promise<void>,
): Promise<FileMutationBatchResult> {
  const succeeded: string[] = [];
  const failures: Array<{ nodeId: string; message: string }> = [];
  for (const nodeId of nodeIds) {
    try {
      await mutate(nodeId);
      succeeded.push(nodeId);
    } catch (error) {
      failures.push({ nodeId, message: trashMutationMessage(error) });
    }
  }
  return { succeeded, failures };
}