import { describe, expect, it } from "vitest";
import type { NamespaceSummaryVM } from "@sparkops/contracts/filesystem";
import { resolveFileSpaceView } from "./file-space-view";

const organizationId = "00000000-0000-4000-8000-000000000001";
const workspaceId = "00000000-0000-4000-8000-000000000002";
const userId = "00000000-0000-4000-8000-000000000003";

function summary(
  namespace_type: NamespaceSummaryVM["namespace_type"],
  id: string,
): NamespaceSummaryVM {
  return {
    id,
    namespace_type,
    organization_id: organizationId,
    workspace_id: namespace_type === "WORKSPACE" ? workspaceId : null,
    owner_user_id: namespace_type === "USER" ? userId : null,
    root_node_id: "00000000-0000-4000-8000-000000000010",
    status: "ACTIVE",
    capabilities: ["READ"],
    entry_node_ids: [],
    entry_nodes_truncated: false,
  };
}

describe("resolveFileSpaceView", () => {
  it.each([
    ["my", "USER"],
    ["workspace", "WORKSPACE"],
    ["organization", "ORGANIZATION"],
  ] as const)("resolves %s only from accessible summaries", (space, namespaceType) => {
    const expected = summary(namespaceType, `00000000-0000-4000-8000-00000000000${namespaceType.length}`);
    const result = resolveFileSpaceView({
      space,
      summaries: [expected],
      organizationId,
      workspaceId,
      userId,
      workspaceIsMember: true,
    });
    expect(result).toEqual({ kind: "namespace", space, namespace: expected });
  });

  it("does not invent a namespace for shared", () => {
    expect(resolveFileSpaceView({
      space: "shared",
      summaries: [summary("WORKSPACE", "00000000-0000-4000-8000-000000000020")],
      organizationId,
      workspaceId,
      userId,
      workspaceIsMember: true,
    })).toEqual({ kind: "shared", space: "shared" });
  });

  it("distinguishes a missing namespace from an unavailable workspace context", () => {
    expect(resolveFileSpaceView({
      space: "my",
      summaries: [],
      organizationId,
      workspaceId,
      userId,
      workspaceIsMember: true,
    })).toEqual({ kind: "missing", space: "my" });
    expect(resolveFileSpaceView({
      space: "workspace",
      summaries: [],
      organizationId,
      workspaceId,
      userId,
      workspaceIsMember: false,
    })).toEqual({ kind: "workspace-unavailable", space: "workspace" });
  });
});
