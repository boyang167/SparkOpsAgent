import { beforeEach, describe, expect, it, vi } from "vitest";
import { BffError } from "@sparkops/contracts";

const mocks = vi.hoisted(() => ({
  createBffContext: vi.fn(),
  listGovernedNamespaces: vi.fn(),
  listGovernedChildren: vi.fn(),
  listSharedWithMe: vi.fn(),
  getEffectiveAccess: vi.fn(),
  createGovernedGrant: vi.fn(),
  createShareRequest: vi.fn(),
}));

vi.mock("@sparkops/bff-core", () => ({
  createBffContext: mocks.createBffContext,
  createTraceId: () => "trace-1",
  ok: (data: unknown, traceId: string, status = 200) => ({ data, traceId, status }),
  fail: (error: unknown, traceId: string) => ({
    error,
    traceId,
    status: error instanceof BffError ? error.status : 500,
  }),
}));

vi.mock("@sparkops/bff-modules/filesystem", () => ({
  listGovernedNamespaces: mocks.listGovernedNamespaces,
  listGovernedChildren: mocks.listGovernedChildren,
  listSharedWithMe: mocks.listSharedWithMe,
  getEffectiveAccess: mocks.getEffectiveAccess,
  createGovernedGrant: mocks.createGovernedGrant,
  createShareRequest: mocks.createShareRequest,
}));

const organizationId = "00000000-0000-4000-8000-000000000001";
const workspaceId = "00000000-0000-4000-8000-000000000002";

function request(
  path: string,
  input: {
    organization?: string;
    workspace?: string;
    idempotencyKey?: string;
    body?: unknown;
  } = {},
) {
  const headers = new Headers();
  if (input.organization) headers.set("X-Organization-ID", input.organization);
  if (input.workspace) headers.set("X-Workspace-ID", input.workspace);
  if (input.idempotencyKey) headers.set("Idempotency-Key", input.idempotencyKey);
  return {
    headers,
    nextUrl: new URL(`http://localhost${path}`),
    json: vi.fn().mockResolvedValue(input.body),
  };
}

describe("governed filesystem Next BFF routes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.createBffContext.mockImplementation(async (req: { headers: Headers }) => ({
      workspaceId: req.headers.get("x-workspace-id") ?? undefined,
    }));
  });

  it("requires organization context on all six governed routes", async () => {
    const namespaces = await import("./namespaces/route");
    const children = await import("./namespaces/[namespaceId]/children/route");
    const shared = await import("./shared-with-me/route");
    const effective = await import("./nodes/[nodeId]/effective-access/route");
    const grants = await import("./nodes/[nodeId]/grants/route");
    const shareRequests = await import("./nodes/[nodeId]/share-requests/route");
    const params = Promise.resolve({ namespaceId: "ns-1", nodeId: "node-1" });

    const responses = await Promise.all([
      namespaces.GET(request("/api/bff/v1/filesystem/namespaces") as never),
      children.GET(
        request("/api/bff/v1/filesystem/namespaces/ns-1/children") as never,
        { params },
      ),
      shared.GET(request("/api/bff/v1/filesystem/shared-with-me") as never),
      effective.GET(
        request("/api/bff/v1/filesystem/nodes/node-1/effective-access") as never,
        { params },
      ),
      grants.POST(
        request("/api/bff/v1/filesystem/nodes/node-1/grants") as never,
        { params },
      ),
      shareRequests.POST(
        request("/api/bff/v1/filesystem/nodes/node-1/share-requests") as never,
        { params },
      ),
    ]);

    expect(responses.map((response) => response.status)).toEqual([400, 400, 400, 400, 400, 400]);
    expect(mocks.listGovernedNamespaces).not.toHaveBeenCalled();
    expect(mocks.listGovernedChildren).not.toHaveBeenCalled();
    expect(mocks.listSharedWithMe).not.toHaveBeenCalled();
    expect(mocks.getEffectiveAccess).not.toHaveBeenCalled();
    expect(mocks.createGovernedGrant).not.toHaveBeenCalled();
    expect(mocks.createShareRequest).not.toHaveBeenCalled();
  });

  it("lists namespaces with optional workspace context", async () => {
    const page = { items: [] };
    mocks.listGovernedNamespaces.mockResolvedValue(page);
    const { GET } = await import("./namespaces/route");

    const response = await GET(
      request("/api/bff/v1/filesystem/namespaces", {
        organization: organizationId,
      }) as never,
    );

    expect(mocks.createBffContext).toHaveBeenCalled();
    expect(mocks.listGovernedNamespaces).toHaveBeenCalledWith(
      expect.objectContaining({ workspaceId: undefined }),
      organizationId,
      undefined,
    );
    expect(response).toMatchObject({ data: page, status: 200 });
  });

  it("forwards namespace params and every children query field", async () => {
    mocks.listGovernedChildren.mockResolvedValue({ items: [], next_cursor: "" });
    const { GET } = await import("./namespaces/[namespaceId]/children/route");
    const req = request(
      "/api/bff/v1/filesystem/namespaces/ns-1/children?parent_id=00000000-0000-4000-8000-000000000003&type=DIRECTORY&q=%E8%AE%BE%E8%AE%A1&cursor=00000000-0000-4000-8000-000000000004&limit=25",
      { organization: organizationId, workspace: workspaceId },
    );

    await GET(req as never, { params: Promise.resolve({ namespaceId: "ns-1" }) });

    expect(mocks.listGovernedChildren).toHaveBeenCalledWith(
      expect.objectContaining({ workspaceId }),
      organizationId,
      workspaceId,
      "ns-1",
      {
        parent_id: "00000000-0000-4000-8000-000000000003",
        type: "DIRECTORY",
        q: "设计",
        cursor: "00000000-0000-4000-8000-000000000004",
        limit: 25,
      },
    );
  });

  it("forwards shared-with-me query and tenant context", async () => {
    mocks.listSharedWithMe.mockResolvedValue({ items: [], next_cursor: "" });
    const { GET } = await import("./shared-with-me/route");
    const req = request(
      "/api/bff/v1/filesystem/shared-with-me?source_type=agent&q=%E5%85%B1%E4%BA%AB&cursor=opaque%2B%2F%3D&limit=10",
      { organization: organizationId, workspace: workspaceId },
    );

    await GET(req as never);

    expect(mocks.listSharedWithMe).toHaveBeenCalledWith(
      expect.objectContaining({ workspaceId }),
      organizationId,
      workspaceId,
      { source_type: "agent", q: "共享", cursor: "opaque+/=", limit: 10 },
    );
  });

  it("forwards effective-access params and maps module errors", async () => {
    const { GET } = await import("./nodes/[nodeId]/effective-access/route");
    const req = request("/api/bff/v1/filesystem/nodes/node-1/effective-access", {
      organization: organizationId,
      workspace: workspaceId,
    });
    mocks.getEffectiveAccess.mockResolvedValueOnce({ decision: "ALLOW" });

    await GET(req as never, { params: Promise.resolve({ nodeId: "node-1" }) });

    expect(mocks.getEffectiveAccess).toHaveBeenCalledWith(
      expect.objectContaining({ workspaceId }),
      organizationId,
      workspaceId,
      "node-1",
    );

    mocks.getEffectiveAccess.mockRejectedValueOnce(
      new BffError("forbidden", "FILESYSTEM_RESOURCE_FORBIDDEN", 403, "upstream-trace"),
    );
    const errorResponse = await GET(req as never, {
      params: Promise.resolve({ nodeId: "node-1" }),
    });
    expect(errorResponse).toMatchObject({ status: 403 });
  });

  it("forwards grant body and idempotency key and returns 201", async () => {
    const body = {
      principal_type: "ROLE",
      principal_ids: ["00000000-0000-4000-8000-000000000005"],
      actions: ["DOWNLOAD"],
    };
    const result = { request: { status: "APPROVED" }, grants: [] };
    mocks.createGovernedGrant.mockResolvedValue(result);
    const { POST } = await import("./nodes/[nodeId]/grants/route");

    const response = await POST(
      request("/api/bff/v1/filesystem/nodes/node-1/grants", {
        organization: organizationId,
        workspace: workspaceId,
        idempotencyKey: "grant-key-1",
        body,
      }) as never,
      { params: Promise.resolve({ nodeId: "node-1" }) },
    );

    expect(mocks.createGovernedGrant).toHaveBeenCalledWith(
      expect.objectContaining({ workspaceId }),
      organizationId,
      workspaceId,
      "node-1",
      body,
      "grant-key-1",
    );
    expect(response).toMatchObject({ data: result, status: 201 });
  });

  it("forwards share-request body and idempotency key and returns 202", async () => {
    const body = {
      principal_type: "USER",
      principal_ids: ["00000000-0000-4000-8000-000000000005"],
      actions: ["DOWNLOAD"],
    };
    const result = { ...body, status: "PENDING" };
    mocks.createShareRequest.mockResolvedValue(result);
    const { POST } = await import("./nodes/[nodeId]/share-requests/route");

    const response = await POST(
      request("/api/bff/v1/filesystem/nodes/node-1/share-requests", {
        organization: organizationId,
        workspace: workspaceId,
        idempotencyKey: "share-key-1",
        body,
      }) as never,
      { params: Promise.resolve({ nodeId: "node-1" }) },
    );

    expect(mocks.createShareRequest).toHaveBeenCalledWith(
      expect.objectContaining({ workspaceId }),
      organizationId,
      workspaceId,
      "node-1",
      body,
      "share-key-1",
    );
    expect(response).toMatchObject({ data: result, status: 202 });
  });
});
