import { describe, expect, it } from "vitest";
import {
  allowedActions,
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
});
