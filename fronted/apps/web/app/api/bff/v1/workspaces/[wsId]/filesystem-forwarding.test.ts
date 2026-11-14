import { beforeEach, describe, expect, it, vi } from "vitest";

const listFileNodes = vi.fn();
const moveFileNode = vi.fn();

vi.mock("@sparkops/bff-core", () => ({
  createBffHandler: (handler: unknown) => handler,
}));

vi.mock("@sparkops/bff-modules/filesystem", () => ({
  listFileNodes,
  moveFileNode,
}));

describe("filesystem Next BFF routes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards type, q and pagination from the files route", async () => {
    const { GET } = await import("./files/route");
    const request = {
      nextUrl: new URL(
        "http://localhost/api/bff/v1/workspaces/workspace-1/files?parent_id=parent-1&type=DIRECTORY&q=%E8%AE%BE%E8%AE%A1&cursor=cursor-1&limit=20",
      ),
    };

    await (GET as unknown as Function)({}, request, {
      params: { wsId: "workspace-1" },
    });

    expect(listFileNodes).toHaveBeenCalledWith({}, "workspace-1", {
      parent_id: "parent-1",
      type: "DIRECTORY",
      q: "设计",
      cursor: "cursor-1",
      limit: 20,
    });
  });

  it("rejects a non-empty unsupported type instead of dropping it", async () => {
    const { GET } = await import("./files/route");
    const request = {
      nextUrl: new URL(
        "http://localhost/api/bff/v1/workspaces/workspace-1/files?type=SYMLINK",
      ),
    };

    await expect(
      (GET as unknown as Function)({}, request, {
        params: { wsId: "workspace-1" },
      }),
    ).rejects.toMatchObject({
      code: "INVALID_REQUEST",
      status: 400,
    });
    expect(listFileNodes).not.toHaveBeenCalled();
  });

  it("forwards move JSON and Idempotency-Key without rewriting", async () => {
    const { POST } = await import("./files/[nodeId]/move/route");
    const input = { parent_id: "parent-2", name: "报告.md" };
    const request = {
      json: vi.fn().mockResolvedValue(input),
      headers: new Headers({ "Idempotency-Key": "move-key-1" }),
    };

    await (POST as unknown as Function)({}, request, {
      params: { wsId: "workspace-1", nodeId: "node-1" },
    });

    expect(moveFileNode).toHaveBeenCalledWith(
      {},
      "workspace-1",
      "node-1",
      input,
      "move-key-1",
    );
  });
});
