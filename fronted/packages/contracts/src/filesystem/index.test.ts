import { describe, expect, it } from "vitest";
import { SharedItemVMSchema } from "./index";

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
        expires_at: null,
      }],
      original_path: "/项目/报告.md",
      source_namespace: {
        namespace_type: "USER",
        owner_user_id: "00000000-0000-4000-8000-000000000006",
      },
    });

    expect(parsed.sources[0]?.principal_type).toBe("AGENT");
    expect(parsed.original_path).toBe("/项目/报告.md");
    expect(parsed.source_namespace.owner_user_id).toBe(
      "00000000-0000-4000-8000-000000000006",
    );
  });
});
