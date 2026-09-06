import { describe, expect, it } from "vitest";
import { cleanedPathname } from "./normalize-pathname";

describe("cleanedPathname", () => {
  it("strips a leading encoded space segment", () => {
    expect(cleanedPathname("/%20/delivery/pipelines")).toBe("/delivery/pipelines");
  });

  it("strips a decoded space segment", () => {
    expect(cleanedPathname("/ /delivery/pipelines")).toBe("/delivery/pipelines");
  });

  it("returns null when the path is already clean", () => {
    expect(cleanedPathname("/delivery/pipelines")).toBeNull();
  });
});
