import { describe, expect, it, vi } from "vitest";
import {
  createPanelRenameHandler,
  submitPanelRename,
} from "./file-explorer-panel-rename";
import { renameNodeKeepingIdentity } from "./file-explorer-rename";

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  renameFileNode: vi.fn(),
  createDirectoryNode: vi.fn(),
}));

/**
 * Panel-level rename wiring (Vitest cannot transform Next `jsx: preserve` panels in this toolchain).
 */
describe("FileExplorerPanel", () => {
  it("keeps file identity stable after rename", async () => {
    const mockRename = vi.fn().mockResolvedValue(undefined);
    await renameNodeKeepingIdentity("file-1", "b.txt", mockRename);
    expect(mockRename).toHaveBeenCalledWith("file-1", "b.txt");
  });

  it("routes panel rename through onRename when injected", async () => {
    const onRename = vi.fn().mockResolvedValue(undefined);
    const rename = createPanelRenameHandler("ws-1", onRename);
    await submitPanelRename("file-1", "  b.txt ", rename);
    expect(onRename).toHaveBeenCalledWith("file-1", "b.txt");
  });

  it("returns trimmed name after successful panel rename", async () => {
    const onRename = vi.fn().mockResolvedValue(undefined);
    const nextName = await submitPanelRename("file-1", "renamed.txt", onRename);
    expect(nextName).toBe("renamed.txt");
  });
});
