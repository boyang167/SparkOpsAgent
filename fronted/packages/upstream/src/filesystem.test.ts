import { afterEach, describe, expect, it, vi } from "vitest";
import { createFilesystemUpstream } from "./filesystem";

describe("filesystem upstream", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("forwards directory and search list filters to the backend", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: { items: [], next_cursor: "" },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await createFilesystemUpstream().listFiles("workspace-1", {
      parent_id: "parent-1",
      type: "DIRECTORY",
      q: "设计",
      cursor: "cursor-1",
      limit: 20,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(
        "/api/v1/workspaces/workspace-1/files?parent_id=parent-1&type=DIRECTORY&q=%E8%AE%BE%E8%AE%A1&cursor=cursor-1&limit=20",
      ),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("forwards move body and idempotency key to the backend contract", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: { updated: true },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await createFilesystemUpstream().moveFileNode(
      "workspace-1",
      "node-1",
      { parent_id: "parent-2", name: "报告.md" },
      "move-key-1",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/workspaces/workspace-1/files/node-1/move"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ parent_id: "parent-2", name: "报告.md" }),
        headers: expect.any(Headers),
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe("move-key-1");
  });
});
