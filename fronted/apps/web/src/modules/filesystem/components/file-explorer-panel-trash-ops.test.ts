import { describe, expect, it, vi } from "vitest";
import { BffError } from "@sparkops/contracts";
import { runTrashBatch, trashMutationMessage } from "./file-explorer-panel-trash-ops";

describe("runTrashBatch", () => {
  it("runs mutations sequentially and returns partial success", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(
        new BffError("original directory already contains an item with this name", "FILESYSTEM_RESTORE_NAME_CONFLICT", 409),
      );

    await expect(runTrashBatch(["node-1", "node-2"], mutate)).resolves.toEqual({
      succeeded: ["node-1"],
      failures: [{ nodeId: "node-2", message: "原目录已有同名项目" }],
    });
    expect(mutate).toHaveBeenNthCalledWith(1, "node-1");
    expect(mutate).toHaveBeenNthCalledWith(2, "node-2");
  });
});

describe("trashMutationMessage", () => {
  it("maps restore conflict codes to user-facing copy", () => {
    expect(
      trashMutationMessage(
        new BffError("conflict", "FILESYSTEM_RESTORE_NAME_CONFLICT", 409),
      ),
    ).toBe("原目录已有同名项目");
  });
});