import { describe, expect, it } from "vitest";
import { validateNodeDrop } from "./directory-dnd";

const validInput = {
  draggedIds: ["dir-a"],
  targetId: "dir-b",
  descendantIds: new Set<string>(),
  sameWorkspace: true,
  canWrite: true,
  allActive: true,
};

describe("validateNodeDrop", () => {
  it("rejects moving a node into itself", () => {
    expect(validateNodeDrop({ ...validInput, targetId: "dir-a" })).toEqual({
      allowed: false,
      reason: "不能移动到自身",
    });
  });

  it("rejects moving a directory into its descendant", () => {
    expect(
      validateNodeDrop({
        ...validInput,
        descendantIds: new Set(["dir-b"]),
      }),
    ).toEqual({ allowed: false, reason: "不能移动到子目录" });
  });

  it.each([
    [{ canWrite: false }, "没有移动权限"],
    [{ sameWorkspace: false }, "不能跨工作区移动"],
    [{ allActive: false }, "不能移动不可用的项目"],
  ])("rejects invalid move context %#", (override, reason) => {
    expect(validateNodeDrop({ ...validInput, ...override })).toEqual({
      allowed: false,
      reason,
    });
  });

  it("allows an active same-workspace move with write permission", () => {
    expect(validateNodeDrop(validInput)).toEqual({ allowed: true });
  });
});
