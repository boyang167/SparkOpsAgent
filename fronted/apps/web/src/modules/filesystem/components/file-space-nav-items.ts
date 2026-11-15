import type { FileSpace } from "./workbench-state";

export const FILE_SPACE_NAV_ITEMS: ReadonlyArray<{ id: FileSpace; label: string }> = [
  { id: "my", label: "我的文件" },
  { id: "workspace", label: "工作区文件" },
  { id: "organization", label: "租户文件" },
  { id: "shared", label: "与我共享" },
];