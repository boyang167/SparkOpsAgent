import { describe, expect, it } from "vitest";
import { cleanedPathname } from "./normalize-pathname";

describe("cleanedPathname", () => {
  it("strips a leading encoded space segment", () => {
    expect(cleanedPathname("/%20/products")).toBe("/products");
  });

  it("strips a decoded space segment", () => {
    expect(cleanedPathname("/ /products")).toBe("/products");
  });

  it("returns null when the path is already clean", () => {
    expect(cleanedPathname("/products")).toBeNull();
  });
});
