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
});
