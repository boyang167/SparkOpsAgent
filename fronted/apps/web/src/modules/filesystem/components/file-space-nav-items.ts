import type { FileSpace } from "./workbench-state";

export const FILE_SPACE_NAV_ITEMS: ReadonlyArray<{ id: FileSpace; label: string }> = [
  { id: "my", label: "我的文件" },
  { id: "workspace", label: "工作区文件" },
  { id: "organization", label: "租户文件" },
  { id: "shared", label: "与我共享" },
];

/** 回收站按命名空间分桶，不含「与我共享」 */
export const TRASH_FILE_SPACE_NAV_ITEMS: ReadonlyArray<{ id: FileSpace; label: string }> = [
  { id: "my", label: "个人空间" },
  { id: "workspace", label: "工作区空间" },
  { id: "organization", label: "租户空间" },
];