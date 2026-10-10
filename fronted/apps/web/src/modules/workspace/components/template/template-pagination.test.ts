import { describe, expect, it } from "vitest";
import { paginateItems } from "./template-pagination";

describe("paginateItems", () => {
  it("returns only the requested page for card and table views", () => {
    const items = Array.from({ length: 45 }, (_, index) => index + 1);

    expect(paginateItems(items, 2, 20)).toEqual(
      Array.from({ length: 20 }, (_, index) => index + 21),
    );
  });

  it("returns the remaining items on the last page", () => {
    const items = Array.from({ length: 45 }, (_, index) => index + 1);

    expect(paginateItems(items, 3, 20)).toEqual([41, 42, 43, 44, 45]);
  });
});
