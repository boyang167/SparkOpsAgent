const TRANSFER_ERROR_MESSAGES: Record<string, string> = {
  object_missing:
    "校验失败：存储上找不到上传的临时文件，请检查 Provider 的 LOCAL Endpoint 是否为可写目录",
  integrity_mismatch: "校验失败：文件大小或校验和不一致",
  name_conflict: "上传失败：当前目录已存在同名文件",
  target_unavailable: "上传失败：目标目录不存在或不可用",
  quota_exceeded: "上传失败：存储配额不足",
};

export function transferErrorMessage(errorCode?: string): string {
  const code = errorCode?.trim();
  return (code && TRANSFER_ERROR_MESSAGES[code]) || code || "上传处理失败";
}
