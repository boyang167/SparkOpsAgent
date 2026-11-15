import { describe, expect, it } from "vitest";
import { fileSpacePath, normalizeFilesPageSearch } from "./page-state";

describe("normalizeFilesPageSearch", () => {
  it("removes an overlong trimmed query and its cursor exactly once", () => {
    const overlong = `  ${"x".repeat(201)}  `;
    const first = normalizeFilesPageSearch({
      tab: "files",
      view: "grid",
      parent: "parent-1",
      cursor: "cursor-1",
      q: overlong,
    });

    expect(first.redirectPath).toBe(
      "/files/data/workspace?view=grid&parent=parent-1",
    );
    expect(first.state.query).toBe("");

    const second = normalizeFilesPageSearch({
      view: "grid",
      parent: "parent-1",
    });
    expect(second.redirectPath).toBeUndefined();
  });

  it("does not let an illegal file query block another tab", () => {
    const result = normalizeFilesPageSearch({
      tab: "transfers",
      q: "x".repeat(201),
      cursor: "cursor-1",
    });

    expect(result.redirectPath).toBeUndefined();
    expect(result.state.tab).toBe("transfers");
  });

  it("normalizes an invalid space to workspace", () => {
    expect(normalizeFilesPageSearch({ space: "invalid" }).state.space).toBe("workspace");
  });

  it("clears scoped navigation while preserving view when changing space", () => {
    expect(
      fileSpacePath("organization", {
        tab: "files",
        view: "grid",
        space: "workspace",
        source: undefined,
        parent: "parent-1",
        cursor: "cursor-1",
        query: "report",
        node: "node-1",
      }),
    ).toBe("/files/data/workspace?view=grid&space=organization&q=report");
  });

  it("normalizes shared sources through the whitelist", () => {
    expect(fileSpacePath("shared", {
      tab: "files",
      view: "list",
      space: "workspace",
      source: undefined,
      query: "",
    }, "organization")).toBe("/files/data/workspace?space=shared&source=organization");
    expect(fileSpacePath("shared", {
      tab: "files",
      view: "list",
      space: "workspace",
      source: undefined,
      query: "",
    }, "external")).toBe("/files/data/workspace?space=shared");
    expect(fileSpacePath("shared", {
      tab: "files",
      view: "list",
      space: "workspace",
      query: "",
    }, "agent")).toBe("/files/data/workspace?space=shared&source=agent");
  });
});
