import { describe, expect, it } from "vitest";
import { BffError } from "@sparkops/contracts";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { isInvalidFilesystemScopeError, resolveFileNodeSelection } from "./filesystem";

const node: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: "00000000-0000-4000-8000-000000000012",
  parent_id: null,
  type: "FILE",
  name: "详情.md",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000013",
};

describe("resolveFileNodeSelection", () => {
  it("returns a loaded node", async () => {
    await expect(resolveFileNodeSelection(() => Promise.resolve(node))).resolves.toEqual({
      node,
    });
  });

  it.each([
    [403, "forbidden"],
    [404, "not-found"],
    [500, "error"],
  ] as const)("maps a %s load failure into drawer state", async (status, error) => {
    await expect(
      resolveFileNodeSelection(() =>
        Promise.reject(new BffError("failed", "FILESYSTEM_ERROR", status))),
    ).resolves.toEqual({ error });
  });

  it("maps an unknown load failure into a retryable drawer error", async () => {
    await expect(
      resolveFileNodeSelection(() => Promise.reject(new Error("offline"))),
    ).resolves.toEqual({ error: "error" });
  });
});

describe("isInvalidFilesystemScopeError", () => {
  it.each([400, 403, 404])("treats %s as a stale URL scope", (status) => {
    expect(
      isInvalidFilesystemScopeError(
        new BffError("stale scope", "FILESYSTEM_ERROR", status),
      ),
    ).toBe(true);
  });

  it("does not hide service failures as stale scope", () => {
    expect(
      isInvalidFilesystemScopeError(
        new BffError("offline", "FILESYSTEM_ERROR", 503),
      ),
    ).toBe(false);
  });
});
