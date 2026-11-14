import { describe, expect, it, vi } from "vitest";
import {
  clearNodeDragSession,
  getActiveNodeDragPayload,
  readNodeDragPayload,
  validateNodeDrop,
  writeNodeDragPayload,
} from "./directory-dnd";

const validInput = {
  draggedIds: ["dir-a"],
  targetId: "dir-b",
  descendantIds: new Set<string>(),
  sameWorkspace: true,
  sameNamespace: true,
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
    [{ sameNamespace: false }, "跨空间请使用复制、转存或采纳"],
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

describe("internal node drag session", () => {
  it("keeps the payload available while DataTransfer is protected and clears it explicitly", () => {
    const dataTransfer = {
      effectAllowed: "none",
      setData: vi.fn(),
      getData: vi.fn(() => {
        throw new Error("protected");
      }),
    } as unknown as DataTransfer;
    const node = {
      id: "file-a",
      namespace_id: "namespace-1",
      parent_id: "source-a",
      type: "FILE",
      name: "A.txt",
      current_revision_id: null,
      state: "ACTIVE",
      owner_id: "owner-1",
    } as const;

    writeNodeDragPayload(dataTransfer, "workspace-1", "namespace-1", [node]);

    expect(readNodeDragPayload(dataTransfer)).toEqual(getActiveNodeDragPayload());
    expect(getActiveNodeDragPayload()?.namespace_id).toBe("namespace-1");
    expect(getActiveNodeDragPayload()?.nodes.map((item) => item.id)).toEqual(["file-a"]);
    clearNodeDragSession();
    expect(getActiveNodeDragPayload()).toBeNull();
  });
});
