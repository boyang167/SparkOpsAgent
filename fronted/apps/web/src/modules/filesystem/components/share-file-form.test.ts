import { describe, expect, it } from "vitest";
import type { CreateGovernedGrantCommand } from "@sparkops/contracts/filesystem";
import {
  allowedActions,
  canonicalSharePayload,
  createInitialShareForm,
  impactSummary,
  submitLabel,
} from "./share-file-form";

describe("share file form", () => {
  it("defaults to READ and DOWNLOAD with directory inheritance enabled", () => {
    expect(createInitialShareForm("DIRECTORY")).toMatchObject({
      actions: ["READ", "DOWNLOAD"],
      inheritToChildren: true,
    });
  });

  it("does not allow destructive or delegating organization actions", () => {
    expect(allowedActions("ORGANIZATION")).not.toEqual(
      expect.arrayContaining(["DELETE", "SHARE", "ADMIN"]),
    );
  });

  it("uses authoritative counts in the exact impact summary", () => {
    expect(
      impactSummary({
        principalType: "WORKSPACE",
        principalId: "workspace-id",
        label: "研发空间",
        memberCount: 17,
      }),
    ).toBe("将影响研发空间的 17 位成员");
    expect(
      impactSummary({
        principalType: "USER",
        principalId: "member-id",
        label: "王小明",
        memberCount: 1,
      }),
    ).toBe("将影响王小明（1 位成员）");
  });

  it("uses an approval-aware submit label", () => {
    expect(submitLabel(false)).toBe("立即共享");
    expect(submitLabel(true)).toBe("提交审批");
  });

  it("fingerprints every logical share payload field canonically", () => {
    const baseline: CreateGovernedGrantCommand = {
      principal_type: "USER",
      principal_ids: ["00000000-0000-4000-8000-000000000001"],
      actions: ["READ", "DOWNLOAD"],
      effect: "ALLOW",
      inherit_to_children: true,
      expires_at: null,
    };
    expect(canonicalSharePayload({
      ...baseline,
      actions: ["DOWNLOAD", "READ"],
    })).toBe(canonicalSharePayload(baseline));

    const edits = [
      { ...baseline, principal_ids: ["00000000-0000-4000-8000-000000000002"] },
      { ...baseline, actions: ["READ"] as CreateGovernedGrantCommand["actions"] },
      { ...baseline, effect: "DENY" as const },
      { ...baseline, inherit_to_children: false },
      { ...baseline, expires_at: "2026-12-31T16:00:00.000Z" },
    ];
    for (const edit of edits) {
      expect(canonicalSharePayload(edit)).not.toBe(canonicalSharePayload(baseline));
    }
  });
});
