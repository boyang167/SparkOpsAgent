import { describe, expect, it } from "vitest";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import {
  formatFileNodeTimestamp,
  resolveMemberDisplayName,
} from "./file-detail-display";

const members: WorkspaceMemberVM[] = [
  {
    member_id: "00000000-0000-4000-8000-000000000013",
    display_name: "张三",
    email: "zhang@example.com",
    roles: [{ id: "role-1", name: "Member", permissions: [] }],
  },
];

describe("file-detail-display", () => {
  it("resolves owner display name from members", () => {
    expect(resolveMemberDisplayName("00000000-0000-4000-8000-000000000013", members)).toBe(
      "张三",
    );
    expect(resolveMemberDisplayName("00000000-0000-4000-8000-000000000099", members)).toBe(null);
  });

  it("formats timestamps for zh-CN locale", () => {
    expect(formatFileNodeTimestamp("2026-11-17T10:30:00.000Z")).not.toBe("—");
    expect(formatFileNodeTimestamp(undefined)).toBe("—");
    expect(formatFileNodeTimestamp("not-a-date")).toBe("—");
  });
});