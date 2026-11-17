import { describe, expect, it } from "vitest";
import type { TransferSessionVM } from "@sparkops/contracts/filesystem";
import type { TransferItem } from "./transfer-queue";
import {
  sliceTransferTablePage,
  TRANSFER_TABLE_PAGE_SIZE,
} from "./transfer-center-pagination";

function persisted(id: string, name: string): TransferSessionVM {
  return {
    session_id: id,
    workspace_id: "ws-1",
    namespace_id: "00000000-0000-0000-0000-000000000002",
    target_parent_id: "00000000-0000-0000-0000-000000000003",
    target_name: name,
    status: "COMPLETED",
    protocol: "DIRECT",
    expected_size: 1,
    part_size: 1,
    total_parts: 1,
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
    parts: [],
  };
}

function localItem(id: string): TransferItem {
  return {
    id,
    file: new File(["x"], `${id}.txt`),
    targetParentId: "parent-1",
    status: "completed",
    progress: { uploadedBytes: 1, totalBytes: 1 },
    cancelRequested: false,
    attempt: 0,
  };
}

describe("sliceTransferTablePage", () => {
  it("returns the first page and keeps local items before persisted ones", () => {
    const locals = [localItem("l1"), localItem("l2")];
    const sessions = Array.from({ length: 11 }, (_, index) =>
      persisted(`p-${index}`, `文件-${index}.txt`),
    );
    const page = sliceTransferTablePage(locals, sessions, 0, TRANSFER_TABLE_PAGE_SIZE);
    expect(page.totalCount).toBe(13);
    expect(page.totalPages).toBe(2);
    expect(page.localItems).toHaveLength(2);
    expect(page.persistedItems).toHaveLength(8);
  });

  it("serves the second page from persisted items only", () => {
    const sessions = Array.from({ length: 12 }, (_, index) =>
      persisted(`p-${index}`, `文件-${index}.txt`),
    );
    const page = sliceTransferTablePage([], sessions, 1, TRANSFER_TABLE_PAGE_SIZE);
    expect(page.pageIndex).toBe(1);
    expect(page.persistedItems).toHaveLength(2);
    expect(page.persistedItems[0]?.target_name).toBe("文件-10.txt");
  });
});