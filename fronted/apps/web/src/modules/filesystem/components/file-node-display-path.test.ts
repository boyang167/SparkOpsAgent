import { describe, expect, it } from "vitest";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { formatNodePath, nodePathInFolder } from "./file-node-display-path";

const file: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: "00000000-0000-4000-8000-000000000002",
  parent_id: "00000000-0000-4000-8000-000000000003",
  type: "FILE",
  name: "K8s.md",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000013",
};

describe("file-node-display-path", () => {
  it("formats folder and file name into a leading-slash path", () => {
    expect(nodePathInFolder("test", file)).toBe("/test/K8s.md");
  });

  it("uses only the node name when the folder is root", () => {
    expect(nodePathInFolder("根目录", file)).toBe("/K8s.md");
    expect(nodePathInFolder(undefined, file)).toBe("/K8s.md");
  });

  it("joins cleaned path segments", () => {
    expect(formatNodePath(["a", " ", "b"])).toBe("/a/b");
  });
});