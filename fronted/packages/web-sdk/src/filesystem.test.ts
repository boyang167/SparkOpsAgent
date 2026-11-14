import { afterEach, describe, expect, it, vi } from "vitest";
import type { TransferSessionVM } from "@sparkops/contracts/filesystem";
import {
  CreateGovernedGrantCommandSchema,
  NamespaceSummaryVMSchema,
  ShareRequestVMSchema,
} from "@sparkops/contracts/filesystem";
import {
  createGovernedGrant,
  createShareRequest,
  getEffectiveAccess,
  getFileNode,
  listGovernedChildren,
  listGovernedNamespaces,
  listGovernedGrants,
  listShareRequests,
  listSharedWithMe,
  listFileAuditEvents,
  listFileNodes,
  listTransferSessions,
  moveFileNode,
  previewGovernedShare,
  uploadTransfer,
} from "./filesystem";

function sessionWithCompletedParts(partNumbers: number[]): TransferSessionVM {
  const partSize = 5;
  const totalParts = 4;
  const parts = Array.from({ length: totalParts }, (_, index) => {
    const partNumber = index + 1;
    const completed = partNumbers.includes(partNumber);
    return {
      part_number: partNumber,
      offset: (partNumber - 1) * partSize,
      size: partSize,
      etag: completed ? `etag-${partNumber}` : "",
      upload_url: completed ? undefined : `https://storage.example/parts/${partNumber}`,
    };
  });
  return {
    session_id: "00000000-0000-0000-0000-000000000099",
    workspace_id: "00000000-0000-0000-0000-000000000001",
    namespace_id: "00000000-0000-0000-0000-000000000002",
    status: "UPLOADING",
    protocol: "MULTIPART",
    target_name: "payload.bin",
    expected_size: totalParts * partSize,
    part_size: partSize,
    total_parts: totalParts,
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
    parts,
  };
}

describe("filesystem transfer sdk", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uploads a direct transfer through the gateway capability contract", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    const session: TransferSessionVM = {
      ...sessionWithCompletedParts([]),
      protocol: "DIRECT",
      upload_url: "https://gateway.example/v1/transfers/session/content",
      capability_token: "upload-capability",
      parts: [],
      total_parts: 1,
    };
    const progress = vi.fn();
    const fileFixture = new Blob(["payload"]);

    const uploaded = await uploadTransfer(fileFixture, {
      session,
      workspaceId: session.workspace_id,
      fetchImpl: fetchMock,
      onProgress: progress,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      session.upload_url,
      expect.objectContaining({
        method: "PUT",
        body: fileFixture,
        headers: { Authorization: "Bearer upload-capability" },
      }),
    );
    expect(progress).toHaveBeenLastCalledWith({
      uploadedBytes: fileFixture.size,
      totalBytes: fileFixture.size,
      completedParts: 1,
      totalParts: 1,
    });
    expect(uploaded.requestedParts).toEqual([1]);
  });

  it("lists persisted workspace transfers with filters and cursor", async () => {
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

    const page = await listTransferSessions("workspace-1", {
      status: "FAILED",
      cursor: "opaque-cursor",
      limit: 20,
    });

    expect(page.next_cursor).toBe("next-page");
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(
        "/workspaces/workspace-1/transfers?status=FAILED&cursor=opaque-cursor&limit=20",
      ),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("uploads only missing parts when resuming", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const fileFixture = new Blob([..."abcdefghijklmnop".split("")]);
    const controller = new AbortController();
    const uploaded = await uploadTransfer(fileFixture, {
      session: sessionWithCompletedParts([1, 2]),
      partSize: 5,
      workspaceId: "00000000-0000-0000-0000-000000000001",
      signal: controller.signal,
    });
    expect(uploaded.requestedParts).toEqual([3, 4]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://storage.example/parts/3");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("https://storage.example/parts/4");
    expect(fetchMock.mock.calls.every(([, init]) => init?.signal === controller.signal)).toBe(true);
  });

  it("stops multipart upload on a non-successful PUT without reporting progress", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("failed", { status: 503 }));
    const progress = vi.fn();

    await expect(
      uploadTransfer(new Blob([..."abcdefghijklmnop".split("")]), {
        session: sessionWithCompletedParts([1, 2]),
        partSize: 5,
        workspaceId: "00000000-0000-0000-0000-000000000001",
        fetchImpl: fetchMock,
        onProgress: progress,
      }),
    ).rejects.toThrow("transfer upload failed: 503");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(progress).not.toHaveBeenCalled();
  });
});

describe("filesystem node sdk", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("encodes a directory type filter after the parent id", async () => {
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

    await listFileNodes("workspace-1", {
      parentId: "parent-1",
      nodeType: "DIRECTORY",
      query: "设计",
      cursor: "cursor-1",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(
        "parent_id=parent-1&type=DIRECTORY&q=%E8%AE%BE%E8%AE%A1&cursor=cursor-1",
      ),
      expect.anything(),
    );
  });

  it("gets one workspace-scoped node for URL selection", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            id: "00000000-0000-4000-8000-000000000011",
            namespace_id: "00000000-0000-4000-8000-000000000002",
            parent_id: null,
            type: "FILE",
            name: "规格.md",
            current_revision_id: null,
            state: "ACTIVE",
            owner_id: "00000000-0000-4000-8000-000000000003",
          },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await getFileNode("workspace-1", "node-1");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/bff/v1/workspaces/workspace-1/files/node-1",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("forwards every supported audit filter and cursor", async () => {
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

    await listFileAuditEvents("workspace-1", {
      actor_id: "00000000-0000-4000-8000-000000000001",
      action: "filesystem.node.rename",
      resource_id: "node-1",
      result: "DENY",
      from: "2026-11-14T00:00:00.000Z",
      to: "2026-11-14T23:59:59.000Z",
      cursor: "20",
      limit: 20,
    });

    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("actor_id=00000000-0000-4000-8000-000000000001");
    expect(url).toContain("action=filesystem.node.rename");
    expect(url).toContain("resource_id=node-1");
    expect(url).toContain("result=DENY");
    expect(url).toContain("cursor=20");
    expect(url).toContain("limit=20");
  });

  it("moves a node with the backend request fields and an idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({ code: 0, message: "ok", data: null, trace_id: "trace-1" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await moveFileNode("workspace-1", "node-1", "target-1", "报告.md", {
      idempotencyKey: "move-key-1",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/bff/v1/workspaces/workspace-1/files/node-1/move",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ parent_id: "target-1", name: "报告.md" }),
        headers: expect.any(Headers),
      }),
    );
    const options = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(options.headers).get("Idempotency-Key")).toBe("move-key-1");
  });

  it("generates non-empty distinct idempotency keys for concurrent moves", async () => {
    vi.spyOn(Date, "now").mockReturnValue(1_700_000_000_000);
    const fetchMock = vi.fn().mockImplementation(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({ code: 0, message: "ok", data: null, trace_id: "trace-1" }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await Promise.all([
      moveFileNode("workspace-1", "node-1", "target-1", "A.txt"),
      moveFileNode("workspace-1", "node-2", "target-1", "B.txt"),
    ]);

    const keys = fetchMock.mock.calls.map(([, init]) =>
      new Headers((init as RequestInit).headers).get("Idempotency-Key"),
    );
    expect(keys[0]).toBeTruthy();
    expect(keys[1]).toBeTruthy();
    expect(keys[0]).not.toBe(keys[1]);
  });
});

describe("governed filesystem sdk", () => {
  it("posts share previews with caller idempotency", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      code: 0,
      message: "ok",
      data: { outcome: "ACTIVE", reason: "POLICY_ACTIVE" },
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    await previewGovernedShare("node-1", {
      principal_type: "USER",
      principal_ids: ["00000000-0000-4000-8000-000000000014"],
      actions: ["READ"],
    }, {
      organizationId: "00000000-0000-4000-8000-000000000001",
      workspaceId: "00000000-0000-4000-8000-000000000002",
      idempotencyKey: "preview-1",
    });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/filesystem/nodes/node-1/share-preview");
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe("preview-1");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("matches the governed namespace and share-request backend contracts", () => {
    const namespace = {
      id: "00000000-0000-4000-8000-000000000001",
      namespace_type: "USER",
      organization_id: "00000000-0000-4000-8000-000000000002",
      workspace_id: null,
      owner_user_id: "00000000-0000-4000-8000-000000000003",
      root_node_id: "00000000-0000-4000-8000-000000000004",
      status: "ACTIVE",
      capabilities: ["READ", "DOWNLOAD", "WRITE", "SHARE"],
      entry_node_ids: [],
      entry_nodes_truncated: false,
    };
    expect(NamespaceSummaryVMSchema.parse(namespace)).toEqual(namespace);
    expect(
      ShareRequestVMSchema.parse({
        id: "00000000-0000-4000-8000-000000000010",
        organization_id: namespace.organization_id,
        namespace_id: namespace.id,
        node_id: namespace.root_node_id,
        requested_by: namespace.owner_user_id,
        principal_type: "USER",
        principal_ids: ["00000000-0000-4000-8000-000000000011"],
        actions: ["READ"],
        inherit_to_children: true,
        expires_at: null,
        classification: "INTERNAL",
        effect: "ALLOW",
        status: "PENDING",
        reviewed_by: null,
        reviewed_at: null,
        rejection_reason: "",
        created_at: "2026-11-15T00:00:00Z",
      }).status,
    ).toBe("PENDING");
  });

  it("rejects duplicate governed principals and actions with clear errors", () => {
    const principal = "00000000-0000-4000-8000-000000000011";
    const duplicatePrincipals = CreateGovernedGrantCommandSchema.safeParse({
      principal_type: "USER",
      principal_ids: [principal, principal],
      actions: ["READ"],
    });
    const duplicateActions = CreateGovernedGrantCommandSchema.safeParse({
      principal_type: "USER",
      principal_ids: [principal],
      actions: ["DOWNLOAD", "DOWNLOAD"],
    });

    expect(duplicatePrincipals.success).toBe(false);
    expect(!duplicatePrincipals.success && duplicatePrincipals.error.issues[0]?.message).toBe(
      "principal_ids must contain unique values",
    );
    expect(duplicateActions.success).toBe(false);
    expect(!duplicateActions.success && duplicateActions.error.issues[0]?.message).toBe(
      "actions must contain unique values",
    );
  });

  it("uses one top-level BFF path and forwards complete read context", async () => {
    const fetchMock = vi.fn().mockImplementation(() =>
      Promise.resolve(new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: { items: [], next_cursor: "" },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      )),
    );
    vi.stubGlobal("fetch", fetchMock);
    const options = {
      organizationId: "00000000-0000-4000-8000-000000000002",
      workspaceId: "00000000-0000-4000-8000-000000000003",
    };

    await listGovernedNamespaces(options);
    await listGovernedChildren(
      "namespace/with space",
      {
        parentId: "00000000-0000-4000-8000-000000000004",
        nodeType: "DIRECTORY",
        query: "设计 文档",
        cursor: "00000000-0000-4000-8000-000000000005",
        limit: 20,
      },
      options,
    );
    await listSharedWithMe({
      sourceType: "agent",
      query: "共享 文档",
      cursor: "opaque+/=",
      limit: 10,
    }, options);
    await getEffectiveAccess("node/with space", options);

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/bff/v1/filesystem/namespaces");
    expect(fetchMock.mock.calls[1]?.[0]).toContain(
      "/api/bff/v1/filesystem/namespaces/namespace%2Fwith%20space/children?parent_id=00000000-0000-4000-8000-000000000004&type=DIRECTORY&q=%E8%AE%BE%E8%AE%A1+%E6%96%87%E6%A1%A3&cursor=00000000-0000-4000-8000-000000000005&limit=20",
    );
    expect(fetchMock.mock.calls[2]?.[0]).toContain(
      "/api/bff/v1/filesystem/shared-with-me?source_type=agent&q=%E5%85%B1%E4%BA%AB+%E6%96%87%E6%A1%A3&cursor=opaque%2B%2F%3D&limit=10",
    );
    expect(fetchMock.mock.calls[3]?.[0]).toBe(
      "/api/bff/v1/filesystem/nodes/node%2Fwith%20space/effective-access",
    );
    for (const [, init] of fetchMock.mock.calls) {
      const headers = new Headers((init as RequestInit).headers);
      expect(headers.get("X-Organization-ID")).toBe(options.organizationId);
      expect(headers.get("X-Workspace-ID")).toBe(options.workspaceId);
    }
  });

  it("returns the governed shared projection through the SDK", async () => {
    const projection = {
      node: {
        id: "00000000-0000-4000-8000-000000000005",
        namespace_id: "00000000-0000-4000-8000-000000000001",
        parent_id: null,
        type: "FILE",
        name: "shared.pdf",
        current_revision_id: null,
        state: "ACTIVE",
        owner_id: "00000000-0000-4000-8000-000000000006",
      },
      effective_actions: ["READ", "DOWNLOAD"],
      source_grant_ids: ["00000000-0000-4000-8000-000000000007"],
      sources: [{
        grant_id: "00000000-0000-4000-8000-000000000007",
        principal_type: "AGENT",
        principal_id: "00000000-0000-4000-8000-000000000008",
        permission: "READ",
        effect: "ALLOW",
        expires_at: "2026-12-31T16:00:00Z",
      }],
      original_path: "/项目/shared.pdf",
      source_namespace: {
        namespace_type: "WORKSPACE",
        organization_id: "00000000-0000-4000-8000-000000000002",
        workspace_id: "00000000-0000-4000-8000-000000000003",
        owner_user_id: null,
      },
    } as const;
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(
      JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [projection], next_cursor: "" },
        trace_id: "trace-1",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    )));

    const page = await listSharedWithMe(
      { sourceType: "agent" },
      {
        organizationId: "00000000-0000-4000-8000-000000000002",
        workspaceId: "00000000-0000-4000-8000-000000000003",
      },
    );

    expect(page.items[0]).toEqual(projection);
  });

  it("forwards governed write body, tenant context, and explicit idempotency key", async () => {
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
    };

    await createGovernedGrant("node/with space", input, {
      organizationId: "00000000-0000-4000-8000-000000000002",
      idempotencyKey: "grant-key-1",
    });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/bff/v1/filesystem/nodes/node%2Fwith%20space/grants");
    expect(init).toEqual(expect.objectContaining({ method: "POST", body: JSON.stringify(input) }));
    const headers = new Headers(init.headers);
    expect(headers.get("X-Organization-ID")).toBe("00000000-0000-4000-8000-000000000002");
    expect(headers.get("Idempotency-Key")).toBe("grant-key-1");
    expect(headers.get("X-Actor-ID")).toBeNull();
  });

  it("reads governed grants and share requests with encoded node ids", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [], classification: "INTERNAL", next_cursor: "", truncated: false },
      }), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [], next_cursor: "", truncated: false },
      }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const options = {
      organizationId: "00000000-0000-4000-8000-000000000002",
      workspaceId: "00000000-0000-4000-8000-000000000003",
    };

    await listGovernedGrants("node/with space", options, { cursor: "opaque+/=", limit: 25 });
    await listShareRequests("node/with space", options, { cursor: "opaque+/=", limit: 25 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/bff/v1/filesystem/nodes/node%2Fwith%20space/grants?cursor=opaque%2B%2F%3D&limit=25",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "/api/bff/v1/filesystem/nodes/node%2Fwith%20space/share-requests?cursor=opaque%2B%2F%3D&limit=25",
    );
  });

  it("posts share requests through the governed route with idempotency", async () => {
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
            actions: ["DOWNLOAD"],
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
      actions: ["DOWNLOAD" as const],
    };

    await createShareRequest("node-1", input, {
      organizationId: "00000000-0000-4000-8000-000000000002",
      workspaceId: "00000000-0000-4000-8000-000000000003",
      idempotencyKey: "share-key-1",
    });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/bff/v1/filesystem/nodes/node-1/share-requests");
    expect(init).toEqual(expect.objectContaining({ method: "POST", body: JSON.stringify(input) }));
    const headers = new Headers(init.headers);
    expect(headers.get("X-Organization-ID")).toBe("00000000-0000-4000-8000-000000000002");
    expect(headers.get("X-Workspace-ID")).toBe("00000000-0000-4000-8000-000000000003");
    expect(headers.get("Idempotency-Key")).toBe("share-key-1");
  });
});
