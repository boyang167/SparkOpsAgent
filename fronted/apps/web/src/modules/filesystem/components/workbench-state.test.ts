import { describe, expect, it } from "vitest";
import { parseWorkbenchState } from "./workbench-state";

describe("parseWorkbenchState", () => {
  it("normalizes unsupported values", () => {
    expect(parseWorkbenchState({ tab: "bad", view: "bad", cursor: "c1" })).toEqual({
      tab: "files",
      view: "list",
      parent: undefined,
      cursor: "c1",
      query: "",
    });
  });

  it("uses the first value from repeated query parameters", () => {
    expect(
      parseWorkbenchState({
        tab: ["audit", "files"],
        view: ["grid", "list"],
        parent: ["parent-1", "parent-2"],
        cursor: ["cursor-1", "cursor-2"],
        q: ["  report  ", "ignored"],
      }),
    ).toEqual({
      tab: "audit",
      view: "grid",
      parent: "parent-1",
      cursor: "cursor-1",
      query: "report",
    });
  });
});
