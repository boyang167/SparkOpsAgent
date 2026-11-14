import { describe, expect, it } from "vitest";
import {
  CreateGovernedGrantCommandSchema,
  GovernanceMetadataQuerySchema,
  SharedItemVMSchema,
} from "./index";

describe("GovernanceMetadataQuerySchema", () => {
  it("clamps requested page sizes to the server maximum", () => {
    expect(GovernanceMetadataQuerySchema.parse({ limit: "999" })).toEqual({ limit: 200 });
  });
});

describe("SharedItemVMSchema", () => {
  it("preserves authoritative source projection metadata", () => {
    const parsed = SharedItemVMSchema.parse({
      node: {
        id: "00000000-0000-4000-8000-000000000001",
        namespace_id: "00000000-0000-4000-8000-000000000002",
        parent_id: null,
        type: "FILE",
        name: "报告.md",
        current_revision_id: null,
        state: "ACTIVE",
        owner_id: "00000000-0000-4000-8000-000000000003",
      },
      effective_actions: ["READ"],
      source_grant_ids: ["00000000-0000-4000-8000-000000000004"],
      sources: [{
        grant_id: "00000000-0000-4000-8000-000000000004",
        principal_type: "AGENT",
        principal_id: "00000000-0000-4000-8000-000000000005",
        permission: "READ",
        effect: "ALLOW",
        expires_at: null,
      }],
      original_path: "/项目/报告.md",
      source_namespace: {
        namespace_type: "USER",
        organization_id: "00000000-0000-4000-8000-000000000007",
        workspace_id: null,
        owner_user_id: "00000000-0000-4000-8000-000000000006",
      },
    });

    expect(parsed.sources[0]?.principal_type).toBe("AGENT");
    expect(parsed.original_path).toBe("/项目/报告.md");
    expect(parsed.source_namespace.owner_user_id).toBe(
      "00000000-0000-4000-8000-000000000006",
    );
  });

  it("keeps a bounded future source type without widening write commands", () => {
    const parsed = SharedItemVMSchema.parse({
      node: {
        id: "00000000-0000-4000-8000-000000000001",
        namespace_id: "00000000-0000-4000-8000-000000000002",
        parent_id: null,
        type: "FILE",
        name: "未来来源.md",
        current_revision_id: null,
        state: "ACTIVE",
        owner_id: "00000000-0000-4000-8000-000000000003",
      },
      effective_actions: ["READ"],
      source_grant_ids: ["00000000-0000-4000-8000-000000000004"],
      sources: [{
        grant_id: "00000000-0000-4000-8000-000000000004",
        principal_type: "FUTURE_PRINCIPAL",
        principal_id: "00000000-0000-4000-8000-000000000005",
        permission: "READ",
        effect: "ALLOW",
      }],
      original_path: "/未来来源.md",
      source_namespace: {
        namespace_type: "WORKSPACE",
        organization_id: "00000000-0000-4000-8000-000000000007",
        workspace_id: "00000000-0000-4000-8000-000000000008",
        owner_user_id: null,
      },
    });

    expect(parsed.sources[0]?.principal_type).toBe("FUTURE_PRINCIPAL");
    expect(parsed.source_namespace.workspace_id).toBe(
      "00000000-0000-4000-8000-000000000008",
    );
    expect(() => SharedItemVMSchema.parse({
      ...parsed,
      sources: [{ ...parsed.sources[0], principal_type: "x".repeat(65) }],
    })).toThrow();
    expect(CreateGovernedGrantCommandSchema.safeParse({
      principal_type: "FUTURE_PRINCIPAL",
      principal_ids: ["00000000-0000-4000-8000-000000000005"],
      actions: ["READ"],
      effect: "ALLOW",
      inherit_to_children: false,
    }).success).toBe(false);
  });
});
