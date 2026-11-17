import { describe, expect, it, vi } from "vitest";
import { deleteDialogCopy, trashPanelNodes } from "./file-explorer-panel-delete";

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  trashFileNode: vi.fn(),
}));

describe("file explorer delete", () => {
  it("describes a single file, a directory, and a selection", () => {
    expect(deleteDialogCopy([{ name: "a.txt", type: "FILE" }])).toEqual({
      title: "移入回收站",
      description: "将文件「a.txt」移入回收站。之后可从回收站恢复。",
    });
    expect(deleteDialogCopy([{ name: "test", type: "DIRECTORY" }])).toEqual({
      title: "移入回收站",
      description: "将目录「test」移入回收站。之后可从回收站恢复。",
    });
    expect(
      deleteDialogCopy([
        { name: "KBs.md", type: "FILE" },
        { name: "test", type: "DIRECTORY" },
      ]).title,
    ).toBe("移入回收站");
  });

  it("keeps nodes that were trashed when a later delete fails", async () => {
    const trash = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("没有删除权限"));

    await expect(trashPanelNodes("ws-1", ["file-1", "file-2"], { trash })).resolves.toEqual({
      ok: false,
      removed: ["file-1"],
      message: "没有删除权限",
    });
    expect(trash).toHaveBeenNthCalledWith(1, "file-1");
    expect(trash).toHaveBeenNthCalledWith(2, "file-2");
  });
});
