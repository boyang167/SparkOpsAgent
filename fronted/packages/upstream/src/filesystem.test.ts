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

  it("forwards trash as DELETE with an empty body and idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await createFilesystemUpstream().trashFileNode(
      "workspace-1",
      "node-1",
      "trash-key-1",
      "namespace-1",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(
        "/api/v1/workspaces/workspace-1/files/node-1?namespace_id=namespace-1",
      ),
      expect.objectContaining({
        method: "DELETE",
        headers: expect.any(Headers),
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(init.body).toBeUndefined();
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe("trash-key-1");
  });

  it("lists trash with namespace, cursor and limit", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: { items: [], next_cursor: "next-1" },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await createFilesystemUpstream().listTrash("workspace-1", {
      namespace_id: "namespace-1",
      cursor: "cursor-1",
      limit: 50,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(
        "/api/v1/workspaces/workspace-1/files/trash?namespace_id=namespace-1&cursor=cursor-1&limit=50",
      ),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("restores and permanently deletes with namespace and idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await createFilesystemUpstream().restoreFileNode(
      "workspace-1",
      "node-1",
      "restore-key-1",
      "namespace-1",
    );
    await createFilesystemUpstream().purgeFileNode(
      "workspace-1",
      "node-2",
      "purge-key-1",
      "namespace-1",
    );

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      expect.stringContaining("/files/node-1/restore?namespace_id=namespace-1"),
      expect.objectContaining({
        method: "POST",
        body: "{}",
        headers: expect.any(Headers),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      expect.stringContaining("/files/node-2/permanent?namespace_id=namespace-1"),
      expect.objectContaining({ method: "DELETE" }),
    );
    const restoreInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const purgeInit = fetchMock.mock.calls[1]?.[1] as RequestInit;
    expect(new Headers(restoreInit.headers).get("Idempotency-Key")).toBe("restore-key-1");
    expect(new Headers(purgeInit.headers).get("Idempotency-Key")).toBe("purge-key-1");
  });

  it("renames with namespace query and idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await createFilesystemUpstream().renameFileNode(
      "workspace-1",
      "node-1",
      { name: "kubeconfig2.yaml" },
      "rename-key-1",
      "namespace-1",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/files/node-1/name?namespace_id=namespace-1"),
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ name: "kubeconfig2.yaml" }),
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe("rename-key-1");
  });

  it("empties trash with DELETE and idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: { items: [{ node_id: "node-1", succeeded: true }] },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await createFilesystemUpstream().emptyTrash(
      "workspace-1",
      "empty-key-1",
      "namespace-1",
    );

    expect(result.items).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/files/trash?namespace_id=namespace-1"),
      expect.objectContaining({ method: "DELETE" }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe("empty-key-1");
  });

  it("forwards governed child filters and tenant context without embedding a workspace path", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: { items: [], next_cursor: "next-page" },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await createFilesystemUpstream().listGovernedChildren(
      {
        organizationId: "00000000-0000-4000-8000-000000000002",
        workspaceId: "00000000-0000-4000-8000-000000000003",
      },
      "namespace/with space",
      {
        parent_id: "00000000-0000-4000-8000-000000000004",
        type: "DIRECTORY",
        q: "设计 文档",
        cursor: "00000000-0000-4000-8000-000000000005",
        limit: 25,
      },
    );

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain(
      "/api/v1/filesystem/namespaces/namespace%2Fwith%20space/children?parent_id=00000000-0000-4000-8000-000000000004&type=DIRECTORY&q=%E8%AE%BE%E8%AE%A1+%E6%96%87%E6%A1%A3&cursor=00000000-0000-4000-8000-000000000005&limit=25",
    );
    expect(url).not.toContain("/workspaces/");
    const headers = new Headers(init.headers);
    expect(headers.get("X-Organization-ID")).toBe("00000000-0000-4000-8000-000000000002");
    expect(headers.get("X-Workspace-ID")).toBe("00000000-0000-4000-8000-000000000003");
  });

  it("uses the governed namespace, shared, and effective-access read transports", async () => {
    const fetchMock = vi.fn().mockImplementation(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            code: 0,
            message: "ok",
            data: { items: [], next_cursor: "" },
            trace_id: "trace-1",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const upstream = createFilesystemUpstream();
    const context = {
      organizationId: "00000000-0000-4000-8000-000000000002",
      workspaceId: "00000000-0000-4000-8000-000000000003",
    };

    await upstream.listGovernedNamespaces(context);
    await upstream.listSharedWithMe(context, {
      source_type: "agent",
      q: "共享 文档",
      cursor: "opaque+/=",
      limit: 10,
    });
    await upstream.getEffectiveAccess(context, "node/with space");

    expect(fetchMock.mock.calls[0]?.[0]).toContain("/api/v1/filesystem/namespaces");
    expect(fetchMock.mock.calls[1]?.[0]).toContain(
      "/api/v1/filesystem/shared-with-me?source_type=agent&q=%E5%85%B1%E4%BA%AB+%E6%96%87%E6%A1%A3&cursor=opaque%2B%2F%3D&limit=10",
    );
    expect(fetchMock.mock.calls[2]?.[0]).toContain(
      "/api/v1/filesystem/nodes/node%2Fwith%20space/effective-access",
    );
    for (const [, init] of fetchMock.mock.calls) {
      expect(init).toEqual(expect.objectContaining({ method: "GET" }));
      const headers = new Headers((init as RequestInit).headers);
      expect(headers.get("X-Organization-ID")).toBe(context.organizationId);
      expect(headers.get("X-Workspace-ID")).toBe(context.workspaceId);
    }
  });

  it("forwards governed grant body, tenant context, and idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            id: "00000000-0000-4000-8000-000000000010",
            organization_id: "00000000-0000-4000-8000-000000000002",
            namespace_id: "00000000-0000-4000-8000-000000000011",
            node_id: "00000000-0000-4000-8000-000000000012",
            requested_by: "00000000-0000-4000-8000-000000000013",
            principal_type: "ROLE",
            principal_ids: ["00000000-0000-4000-8000-000000000014"],
            actions: ["DOWNLOAD"],
            classification: "INTERNAL",
            effect: "ALLOW",
            status: "PENDING",
            created_at: "2026-11-15T00:00:00Z",
          },
          trace_id: "trace-1",
        }),
        { status: 202, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const input = {
      principal_type: "ROLE" as const,
      principal_ids: ["00000000-0000-4000-8000-000000000014"],
      actions: ["DOWNLOAD" as const],
    };

    await createFilesystemUpstream().createGovernedGrant(
      {
        organizationId: "00000000-0000-4000-8000-000000000002",
        workspaceId: "00000000-0000-4000-8000-000000000003",
      },
      "node-1",
      input,
      "grant-key-1",
    );

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/v1/filesystem/nodes/node-1/grants");
    expect(init).toEqual(expect.objectContaining({ method: "POST", body: JSON.stringify(input) }));
    const headers = new Headers(init.headers);
    expect(headers.get("X-Organization-ID")).toBe("00000000-0000-4000-8000-000000000002");
    expect(headers.get("X-Workspace-ID")).toBe("00000000-0000-4000-8000-000000000003");
    expect(headers.get("Idempotency-Key")).toBe("grant-key-1");
  });

  it("forwards governed share body, tenant context, and idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            id: "00000000-0000-4000-8000-000000000010",
            organization_id: "00000000-0000-4000-8000-000000000002",
            namespace_id: "00000000-0000-4000-8000-000000000011",
            node_id: "00000000-0000-4000-8000-000000000012",
            requested_by: "00000000-0000-4000-8000-000000000013",
            principal_type: "USER",
            principal_ids: ["00000000-0000-4000-8000-000000000014"],
            actions: ["READ"],
            classification: "CONFIDENTIAL",
            effect: "ALLOW",
            status: "PENDING",
            created_at: "2026-11-15T00:00:00Z",
          },
          trace_id: "trace-1",
        }),
        { status: 202, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const input = {
      principal_type: "USER" as const,
      principal_ids: ["00000000-0000-4000-8000-000000000014"],
      actions: ["READ" as const],
      effect: "ALLOW" as const,
      inherit_to_children: true,
      expires_at: null,
    };

    await createFilesystemUpstream().createShareRequest(
      { organizationId: "00000000-0000-4000-8000-000000000002" },
      "node/with space",
      input,
      "share-key-1",
    );

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/v1/filesystem/nodes/node%2Fwith%20space/share-requests");
    expect(init).toEqual(expect.objectContaining({ method: "POST", body: JSON.stringify(input) }));
    const headers = new Headers(init.headers);
    expect(headers.get("X-Organization-ID")).toBe("00000000-0000-4000-8000-000000000002");
    expect(headers.get("X-Workspace-ID")).toBeNull();
    expect(headers.get("Idempotency-Key")).toBe("share-key-1");
  });

  it("forwards governed share preview without changing its payload", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      code: 0,
      message: "ok",
      data: { outcome: "ACTIVE", reason: "POLICY_ACTIVE" },
      trace_id: "trace-1",
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const input = {
      principal_type: "USER" as const,
      principal_ids: ["00000000-0000-4000-8000-000000000014"],
      actions: ["READ" as const],
    };

    await createFilesystemUpstream().previewGovernedShare(
      {
        organizationId: "00000000-0000-4000-8000-000000000002",
        workspaceId: "00000000-0000-4000-8000-000000000003",
      },
      "node-1",
      input,
      "preview-key-1",
    );

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/v1/filesystem/nodes/node-1/share-preview");
    expect(init).toEqual(expect.objectContaining({ method: "POST", body: JSON.stringify(input) }));
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe("preview-key-1");
  });

  it("forwards governance cursor pagination to metadata reads", async () => {
    const fetchMock = vi.fn().mockImplementation(async () =>
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [], next_cursor: "", truncated: false },
        trace_id: "trace-1",
      }), { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const context = {
      organizationId: "00000000-0000-4000-8000-000000000002",
      workspaceId: "00000000-0000-4000-8000-000000000003",
    };

    await createFilesystemUpstream().listGovernedGrants(
      context,
      "node/with space",
      { cursor: "opaque+/=", limit: 25 },
    );
    await createFilesystemUpstream().listShareRequests(
      context,
      "node/with space",
      { cursor: "opaque+/=", limit: 25 },
    );

    expect(fetchMock.mock.calls[0]?.[0]).toContain(
      "/api/v1/filesystem/nodes/node%2Fwith%20space/grants?cursor=opaque%2B%2F%3D&limit=25",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toContain(
      "/api/v1/filesystem/nodes/node%2Fwith%20space/share-requests?cursor=opaque%2B%2F%3D&limit=25",
    );
  });
});
