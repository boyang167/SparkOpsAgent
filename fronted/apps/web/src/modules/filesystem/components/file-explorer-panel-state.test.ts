import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { describe, expect, it, vi } from "vitest";
import {
  createFileBrowserMoveAdapter,
  showCreateDialog,
  showDeleteDialog,
  showRenameDialog,
} from "./file-explorer-panel-state";

const node: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: "00000000-0000-4000-8000-000000000001",
  parent_id: "00000000-0000-4000-8000-000000000002",
  type: "FILE",
  name: "规格说明.md",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000003",
};

describe("file explorer panel state", () => {
  it("uses one dialog state so create and rename are mutually exclusive", () => {
    let dialog = showRenameDialog(node);
    expect(dialog).toEqual({ type: "rename", node });

    dialog = showCreateDialog();
    expect(dialog).toEqual({ type: "create" });

    dialog = showRenameDialog(node);
    expect(dialog).toEqual({ type: "rename", node });

    dialog = showDeleteDialog([node]);
    expect(dialog).toEqual({ type: "delete", nodes: [node] });
  });

  it("exposes move only when the owner supplies a move capability", () => {
    const unavailable = createFileBrowserMoveAdapter();
    expect(unavailable.canMove).toBe(false);

    const onMove = vi.fn();
    const available = createFileBrowserMoveAdapter(onMove);
    available.onMove(node);

    expect(available.canMove).toBe(true);
    expect(onMove).toHaveBeenCalledWith(node.id);
  });
});
