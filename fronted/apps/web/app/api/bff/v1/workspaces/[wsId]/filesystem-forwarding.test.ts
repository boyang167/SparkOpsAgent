import { beforeEach, describe, expect, it, vi } from "vitest";

const listFileNodes = vi.fn();
const listTrashItems = vi.fn();
const moveFileNode = vi.fn();
const trashFileNode = vi.fn();
const restoreFileNode = vi.fn();
const purgeFileNode = vi.fn();
const emptyTrash = vi.fn();

vi.mock("@sparkops/bff-core", () => ({
  createBffHandler: (handler: unknown) => handler,
}));

vi.mock("@sparkops/bff-modules/filesystem", () => ({
  listFileNodes,
  listTrashItems,
  moveFileNode,
  trashFileNode,
  restoreFileNode,
  purgeFileNode,
  emptyTrash,
  getFileNode: vi.fn(),
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

  it("forwards trash with Idempotency-Key and no body", async () => {
    const { DELETE } = await import("./files/[nodeId]/route");
    const request = {
      headers: new Headers({ "Idempotency-Key": "trash-key-1" }),
      nextUrl: new URL(
        "http://localhost/api/bff/v1/workspaces/workspace-1/files/node-1?namespace_id=namespace-1",
      ),
    };

    await expect(
      (DELETE as unknown as Function)({}, request, {
        params: { wsId: "workspace-1", nodeId: "node-1" },
      }),
    ).resolves.toEqual({ deleted: true });

    expect(trashFileNode).toHaveBeenCalledWith(
      {},
      "workspace-1",
      "node-1",
      "trash-key-1",
      "namespace-1",
    );
  });

  it("forwards trash list filters from the trash route", async () => {
    const { GET } = await import("./files/trash/route");
    const request = {
      nextUrl: new URL(
        "http://localhost/api/bff/v1/workspaces/workspace-1/files/trash?namespace_id=namespace-1&cursor=cursor-1&limit=50",
      ),
    };

    await (GET as unknown as Function)({}, request, {
      params: { wsId: "workspace-1" },
    });

    expect(listTrashItems).toHaveBeenCalledWith({}, "workspace-1", {
      namespace_id: "namespace-1",
      cursor: "cursor-1",
      limit: 50,
    });
  });

  it("forwards empty trash with Idempotency-Key and namespace", async () => {
    const { DELETE } = await import("./files/trash/route");
    const request = {
      headers: new Headers({ "Idempotency-Key": "empty-key-1" }),
      nextUrl: new URL(
        "http://localhost/api/bff/v1/workspaces/workspace-1/files/trash?namespace_id=namespace-1",
      ),
    };

    await (DELETE as unknown as Function)({}, request, {
      params: { wsId: "workspace-1" },
    });

    expect(emptyTrash).toHaveBeenCalledWith({}, "workspace-1", "empty-key-1", "namespace-1");
  });

  it("forwards restore with Idempotency-Key and namespace", async () => {
    const { POST } = await import("./files/[nodeId]/restore/route");
    const request = {
      headers: new Headers({ "Idempotency-Key": "restore-key-1" }),
      nextUrl: new URL(
        "http://localhost/api/bff/v1/workspaces/workspace-1/files/node-1/restore?namespace_id=namespace-1",
      ),
    };

    await expect(
      (POST as unknown as Function)({}, request, {
        params: { wsId: "workspace-1", nodeId: "node-1" },
      }),
    ).resolves.toEqual({ restored: true });

    expect(restoreFileNode).toHaveBeenCalledWith(
      {},
      "workspace-1",
      "node-1",
      "restore-key-1",
      "namespace-1",
    );
  });

  it("forwards permanent delete with Idempotency-Key and namespace", async () => {
    const { DELETE } = await import("./files/[nodeId]/permanent/route");
    const request = {
      headers: new Headers({ "Idempotency-Key": "purge-key-1" }),
      nextUrl: new URL(
        "http://localhost/api/bff/v1/workspaces/workspace-1/files/node-1/permanent?namespace_id=namespace-1",
      ),
    };

    await expect(
      (DELETE as unknown as Function)({}, request, {
        params: { wsId: "workspace-1", nodeId: "node-1" },
      }),
    ).resolves.toEqual({ purged: true });

    expect(purgeFileNode).toHaveBeenCalledWith(
      {},
      "workspace-1",
      "node-1",
      "purge-key-1",
      "namespace-1",
    );
  });
});
