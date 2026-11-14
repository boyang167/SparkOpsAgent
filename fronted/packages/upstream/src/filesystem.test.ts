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
});
