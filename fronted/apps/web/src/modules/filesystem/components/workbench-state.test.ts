import { describe, expect, it } from "vitest";
import { parseWorkbenchState } from "./workbench-state";

describe("parseWorkbenchState", () => {
  it("normalizes unsupported values", () => {
    expect(parseWorkbenchState({ tab: "bad", view: "bad", cursor: "c1" })).toEqual({
      tab: "files",
      view: "list",
      space: "workspace",
      source: undefined,
      parent: undefined,
      cursor: "c1",
      query: "",
      node: undefined,
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
        node: ["node-1", "node-2"],
      space: ["organization", "my"],
      source: ["workspace", "organization"],
      }),
    ).toEqual({
      tab: "audit",
      view: "grid",
      space: "organization",
      source: undefined,
      parent: "parent-1",
      cursor: "cursor-1",
      query: "report",
      node: "node-1",
    });
  });

  it("allows only whitelisted shared sources and removes parent outside real spaces", () => {
    expect(parseWorkbenchState({ space: "shared", source: "organization", parent: "p1" })).toMatchObject({
      space: "shared",
      source: "organization",
      parent: undefined,
    });
    expect(parseWorkbenchState({ space: "shared", source: "external" })).toMatchObject({
      space: "shared",
      source: undefined,
    });
    expect(parseWorkbenchState({ space: "my", source: "workspace" })).toMatchObject({
      space: "my",
      source: undefined,
    });
  });
});
