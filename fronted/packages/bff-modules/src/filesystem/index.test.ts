import { describe, expect, it, vi } from "vitest";
import type { BffContext } from "@sparkops/bff-core";
import {
  createGovernedGrant,
  createShareRequest,
  getEffectiveAccess,
  listFileNodes,
  listGovernedChildren,
  listGovernedNamespaces,
  listSharedWithMe,
  moveFileNode,
} from "./index";

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

  it("passes governed query and organization/workspace context to upstream", async () => {
    const listChildren = vi.fn().mockResolvedValue({ items: [], next_cursor: "next-page" });
    const query = {
      parent_id: "00000000-0000-4000-8000-000000000004",
      type: "DIRECTORY" as const,
      q: "设计",
      cursor: "00000000-0000-4000-8000-000000000005",
      limit: 20,
    };

    await listGovernedChildren(
      context({ listGovernedChildren: listChildren }),
      "00000000-0000-4000-8000-000000000002",
      "00000000-0000-4000-8000-000000000003",
      "00000000-0000-4000-8000-000000000006",
      query,
    );

    expect(listChildren).toHaveBeenCalledWith(
      {
        organizationId: "00000000-0000-4000-8000-000000000002",
        workspaceId: "00000000-0000-4000-8000-000000000003",
      },
      "00000000-0000-4000-8000-000000000006",
      query,
    );
  });

  it("validates governed grant responses and forwards writes unchanged", async () => {
    const createGrant = vi.fn().mockResolvedValue({
      request: {
        id: "00000000-0000-4000-8000-000000000010",
        organization_id: "00000000-0000-4000-8000-000000000002",
        namespace_id: "00000000-0000-4000-8000-000000000011",
        node_id: "00000000-0000-4000-8000-000000000012",
        requested_by: "00000000-0000-4000-8000-000000000013",
        principal_type: "ROLE",
        principal_ids: ["00000000-0000-4000-8000-000000000014"],
        actions: ["READ"],
        classification: "INTERNAL",
        effect: "ALLOW",
        status: "APPROVED",
        created_at: "2026-11-15T00:00:00Z",
      },
      grants: [
        {
          id: "00000000-0000-4000-8000-000000000015",
          node_id: "00000000-0000-4000-8000-000000000012",
          grantee_id: "00000000-0000-4000-8000-000000000014",
          principal_type: "ROLE",
          principal_id: "00000000-0000-4000-8000-000000000014",
          permission: "READ",
          effect: "ALLOW",
          expires_at: null,
        },
      ],
    });
    const input = {
      principal_type: "ROLE" as const,
      principal_ids: ["00000000-0000-4000-8000-000000000014"],
      actions: ["READ" as const],
    };

    const result = await createGovernedGrant(
      context({ createGovernedGrant: createGrant }),
      "00000000-0000-4000-8000-000000000002",
      undefined,
      "00000000-0000-4000-8000-000000000012",
      input,
      "grant-key-1",
    );

    expect("grants" in result && result.grants[0]?.principal_type).toBe("ROLE");
    expect(createGrant).toHaveBeenCalledWith(
      { organizationId: "00000000-0000-4000-8000-000000000002" },
      "00000000-0000-4000-8000-000000000012",
      input,
      "grant-key-1",
    );
  });

  it("validates and forwards all governed read transports including DOWNLOAD", async () => {
    const listNamespaces = vi.fn().mockResolvedValue({
      items: [{
        id: "00000000-0000-4000-8000-000000000001",
        namespace_type: "WORKSPACE",
        organization_id: "00000000-0000-4000-8000-000000000002",
        workspace_id: "00000000-0000-4000-8000-000000000003",
        owner_user_id: null,
        root_node_id: "00000000-0000-4000-8000-000000000004",
        status: "ACTIVE",
        capabilities: ["READ", "DOWNLOAD"],
        entry_node_ids: [],
        entry_nodes_truncated: false,
      }],
    });
    const listShared = vi.fn().mockResolvedValue({
      items: [{
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
      }],
      next_cursor: "",
    });
    const effectiveAccess = vi.fn().mockResolvedValue({
      decision: "ALLOW",
      effective_actions: ["READ", "DOWNLOAD"],
      reason_code: "EXPLICIT_ALLOW",
      source_grant_ids: ["00000000-0000-4000-8000-000000000007"],
      policy_version: 2,
      cache_key: {
        organization_id: "00000000-0000-4000-8000-000000000002",
        workspace_id: "00000000-0000-4000-8000-000000000003",
        actor_id: "00000000-0000-4000-8000-000000000008",
        node_id: "00000000-0000-4000-8000-000000000005",
        action: "DOWNLOAD",
        policy_version: 2,
      },
    });
    const ctx = context({
      listGovernedNamespaces: listNamespaces,
      listSharedWithMe: listShared,
      getEffectiveAccess: effectiveAccess,
    });
    const organizationId = "00000000-0000-4000-8000-000000000002";
    const workspaceId = "00000000-0000-4000-8000-000000000003";

    const namespaces = await listGovernedNamespaces(ctx, organizationId, workspaceId);
    const shared = await listSharedWithMe(ctx, organizationId, workspaceId, {
      q: "shared",
      cursor: "opaque",
      limit: 20,
    });
    const access = await getEffectiveAccess(
      ctx,
      organizationId,
      workspaceId,
      "00000000-0000-4000-8000-000000000005",
    );

    expect(namespaces.items[0]?.capabilities).toContain("DOWNLOAD");
    expect(shared.items[0]?.effective_actions).toContain("DOWNLOAD");
    expect(access.effective_actions).toContain("DOWNLOAD");
    expect(listNamespaces).toHaveBeenCalledWith({ organizationId, workspaceId });
    expect(listShared).toHaveBeenCalledWith(
      { organizationId, workspaceId },
      { q: "shared", cursor: "opaque", limit: 20 },
    );
    expect(effectiveAccess).toHaveBeenCalledWith(
      { organizationId, workspaceId },
      "00000000-0000-4000-8000-000000000005",
    );
  });

  it("validates and forwards governed share requests unchanged", async () => {
    const submitShare = vi.fn().mockResolvedValue({
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
    });
    const input = {
      principal_type: "USER" as const,
      principal_ids: ["00000000-0000-4000-8000-000000000014"],
      actions: ["DOWNLOAD" as const],
    };

    const result = await createShareRequest(
      context({ createShareRequest: submitShare }),
      "00000000-0000-4000-8000-000000000002",
      "00000000-0000-4000-8000-000000000003",
      "00000000-0000-4000-8000-000000000012",
      input,
      "share-key-1",
    );

    expect("status" in result && result.status).toBe("PENDING");
    expect(submitShare).toHaveBeenCalledWith(
      {
        organizationId: "00000000-0000-4000-8000-000000000002",
        workspaceId: "00000000-0000-4000-8000-000000000003",
      },
      "00000000-0000-4000-8000-000000000012",
      input,
      "share-key-1",
    );
  });
});
