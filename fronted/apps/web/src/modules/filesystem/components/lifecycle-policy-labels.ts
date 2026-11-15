export const LIFECYCLE_WHEN_LABELS: Record<
  "scope" | "classification" | "mime_prefix" | "min_size" | "max_age_days",
  string
> = {
  scope: "作用域",
  classification: "分级",
  mime_prefix: "MIME 前缀",
  min_size: "最小大小（字节）",
  max_age_days: "最大保留天数",
};

export const LIFECYCLE_ACTION_LABELS: Record<string, string> = {
  replicate: "复制",
  archive: "归档",
  trash: "回收站",
  purge: "清除",
  rescan: "重扫",
};