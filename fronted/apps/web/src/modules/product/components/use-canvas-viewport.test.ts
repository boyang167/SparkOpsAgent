import { describe, expect, it } from "vitest";
import { clampScale } from "./use-canvas-viewport";

describe("clampScale", () => {
  it("clamps within bounds", () => {
    expect(clampScale(2)).toBe(1.5);
    expect(clampScale(0.2)).toBe(0.5);
    expect(clampScale(1)).toBe(1);
  });
});
