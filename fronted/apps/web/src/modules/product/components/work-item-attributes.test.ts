import { describe, expect, it } from "vitest";
import { workItemMetaParts } from "./work-item-attributes";

describe("workItemMetaParts", () => {
  it("hides default type and priority", () => {
    expect(workItemMetaParts({ type: "story", priority: "medium" })).toEqual([]);
  });

  it("shows non-default values with labels", () => {
    expect(workItemMetaParts({ type: "bug", priority: "high" }).map((part) => part.label)).toEqual([
      "缺陷",
      "高优先级",
    ]);
  });
});
