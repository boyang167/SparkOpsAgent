import { describe, expect, it, vi } from "vitest";
import type { BffContext } from "@sparkops/bff-core";
import {
  createGovernedGrant,
  listFileNodes,
  listGovernedChildren,
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
});
