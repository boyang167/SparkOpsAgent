import { describe, expect, it, vi } from "vitest";
import { BffError } from "@sparkops/contracts";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import {
  loadAccessibleFileNamespaces,
  loadGovernedFileChildren,
  loadSharedFileItems,
  resolveFileNodeSelection,
  resolveFileSpaceLoad,
  isInvalidFilesystemScopeError,
} from "./filesystem";

const governed = vi.hoisted(() => ({
  namespaces: vi.fn(),
  children: vi.fn(),
  shared: vi.fn(),
}));

vi.mock("@sparkops/bff-modules/filesystem", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@sparkops/bff-modules/filesystem")>()),
  listGovernedNamespaces: governed.namespaces,
  listGovernedChildren: governed.children,
  listSharedWithMe: governed.shared,
}));

vi.mock("./context", () => ({
  loadBffContext: vi.fn().mockResolvedValue({}),
}));

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
    await expect(resolveFileNodeSelection(node.namespace_id, () => Promise.resolve(node))).resolves.toEqual({
      node,
    });
  });

  it("rejects a node loaded from another namespace", async () => {
    await expect(
      resolveFileNodeSelection(
        "00000000-0000-4000-8000-000000000099",
        () => Promise.resolve(node),
      ),
    ).resolves.toEqual({ error: "not-found" });
  });

  it.each([
    [403, "forbidden"],
    [404, "not-found"],
    [500, "error"],
  ] as const)("maps a %s load failure into drawer state", async (status, error) => {
    await expect(
      resolveFileNodeSelection(node.namespace_id, () =>
        Promise.reject(new BffError("failed", "FILESYSTEM_ERROR", status))),
    ).resolves.toEqual({ error });
  });

  it("maps an unknown load failure into a retryable drawer error", async () => {
    await expect(
      resolveFileNodeSelection(node.namespace_id, () => Promise.reject(new Error("offline"))),
    ).resolves.toEqual({ error: "error" });
  });
});

describe("isInvalidFilesystemScopeError", () => {
  it.each([
    [400, "FILESYSTEM_INVALID_REQUEST"],
    [403, "FILESYSTEM_RESOURCE_FORBIDDEN"],
    [404, "FILESYSTEM_NOT_FOUND"],
  ] as const)("treats only %s %s as a stale URL scope", (status, code) => {
    expect(
      isInvalidFilesystemScopeError(
        new BffError("stale scope", code, status),
      ),
    ).toBe(true);
  });

  it.each([
    [400, "SOME_OTHER_BAD_REQUEST"],
    [403, "AUTH_FORBIDDEN"],
    [404, "SOME_OTHER_NOT_FOUND"],
    [503, "FILESYSTEM_UNAVAILABLE"],
  ] as const)("does not hide unrelated %s %s failures", (status, code) => {
    expect(isInvalidFilesystemScopeError(new BffError("failed", code, status))).toBe(
      false,
    );
  });
});

describe("governed filesystem loaders", () => {
  it("loads accessible summaries without substituting a root namespace", async () => {
    governed.namespaces.mockResolvedValueOnce({ items: [] });
    await expect(
      loadAccessibleFileNamespaces("org-1", "workspace-1"),
    ).resolves.toEqual({ items: [] });
    expect(governed.children).not.toHaveBeenCalled();
  });

  it("routes real namespace and shared projections to distinct APIs", async () => {
    governed.children.mockResolvedValueOnce({ items: [], next_cursor: "" });
    governed.shared.mockResolvedValueOnce({ items: [], next_cursor: "" });
    await loadGovernedFileChildren("org-1", "workspace-1", "namespace-1", {
      parentId: "parent-1",
    });
    await loadSharedFileItems("org-1", "workspace-1", {
      source: "agent",
      cursor: "cursor-1",
      query: "report",
      limit: 25,
    });
    expect(governed.children).toHaveBeenCalledTimes(1);
    expect(governed.shared).toHaveBeenCalledWith(
      {},
      "org-1",
      "workspace-1",
      {
        source_type: "agent",
        cursor: "cursor-1",
        q: "report",
        limit: 25,
      },
    );
  });

  it.each([
    [403, "forbidden"],
    [404, "not-found"],
    [500, "error"],
  ] as const)("keeps %s loading failures renderable", async (status, expected) => {
    await expect(resolveFileSpaceLoad(() => Promise.reject(
      new BffError("failed", "FILESYSTEM_ERROR", status),
    ))).resolves.toEqual({ kind: expected });
  });

  it("keeps an empty children page distinct from a missing namespace", async () => {
    await expect(resolveFileSpaceLoad(() => Promise.resolve({
      items: [],
      next_cursor: "",
    }))).resolves.toEqual({
      kind: "loaded",
      value: { items: [], next_cursor: "" },
    });
  });
});
