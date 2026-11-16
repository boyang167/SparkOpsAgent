import { describe, expect, it, vi } from "vitest";
import { deleteDialogCopy, trashPanelNodes } from "./file-explorer-panel-delete";

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  trashFileNode: vi.fn(),
}));

describe("file explorer delete", () => {
  it("describes a single file, a directory, and a selection", () => {
    expect(deleteDialogCopy([{ name: "KBs.md", type: "FILE" }])).toEqual({
      title: "删除文件",
      description: "将删除文件「KBs.md」。删除后它会从当前列表中移除。",
    });
    expect(deleteDialogCopy([{ name: "test", type: "DIRECTORY" }])).toEqual({
      title: "删除目录",
      description: "将删除目录「test」。删除后它会从当前列表中移除。",
    });
    expect(
      deleteDialogCopy([
        { name: "KBs.md", type: "FILE" },
        { name: "test", type: "DIRECTORY" },
      ]).title,
    ).toBe("删除所选项目");
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
