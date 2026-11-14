import { describe, expect, it } from "vitest";
import { normalizeFilesPageSearch } from "./page-state";

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
      "/files?view=grid&parent=parent-1",
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

    expect(result.redirectPath).toBe("/files?tab=transfers");
    expect(result.state.tab).toBe("transfers");
  });
});
