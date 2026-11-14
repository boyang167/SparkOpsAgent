import { describe, expect, it, vi } from "vitest";
import type { BffContext } from "@sparkops/bff-core";
import { listFileNodes, moveFileNode } from "./index";

function context(filesystem: Record<string, unknown>): BffContext {
  return {
    traceId: "trace-1",
    upstream: { filesystem },
  } as unknown as BffContext;
}

describe("filesystem BFF module", () => {
  it("passes all file list filters to upstream", async () => {
    const listFiles = vi.fn().mockResolvedValue({ items: [], next_cursor: "" });

    await listFileNodes(context({ listFiles }), "workspace-1", {
      parent_id: "parent-1",
      type: "DIRECTORY",
      q: "设计",
      cursor: "cursor-1",
      limit: 20,
    });

    expect(listFiles).toHaveBeenCalledWith("workspace-1", {
      parent_id: "parent-1",
      type: "DIRECTORY",
      q: "设计",
      cursor: "cursor-1",
      limit: 20,
    });
  });

  it("passes the real move contract and idempotency key to upstream", async () => {
    const upstreamMove = vi.fn().mockResolvedValue(undefined);

    await moveFileNode(
      context({ moveFileNode: upstreamMove }),
      "workspace-1",
      "node-1",
      { parent_id: "parent-2", name: "报告.md" },
      "move-key-1",
    );

    expect(upstreamMove).toHaveBeenCalledWith(
      "workspace-1",
      "node-1",
      { parent_id: "parent-2", name: "报告.md" },
      "move-key-1",
    );
  });
});
