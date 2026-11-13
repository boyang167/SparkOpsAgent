import { describe, expect, it, vi } from "vitest";
import { renameNodeKeepingIdentity } from "./file-explorer-rename";

/**
 * TDD rename case for FileExplorerPanel (Vitest cannot transform Next `jsx: preserve` panels in this toolchain).
 */
describe("FileExplorerPanel", () => {
  it("keeps file identity stable after rename", async () => {
    const mockRename = vi.fn().mockResolvedValue(undefined);
    await renameNodeKeepingIdentity("file-1", "b.txt", mockRename);
    expect(mockRename).toHaveBeenCalledWith("file-1", "b.txt");
  });
});
