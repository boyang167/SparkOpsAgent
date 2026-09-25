import { describe, expect, it } from "vitest";
import { computeFitViewport } from "./use-canvas-viewport";

describe("computeFitViewport", () => {
  it("centers content in the viewport", () => {
    const viewport = computeFitViewport(1000, 600, 400, 200, 40);
    expect(viewport.scale).toBe(1);
    expect(viewport.offsetX).toBe(300);
    expect(viewport.offsetY).toBe(200);
  });

  it("scales down when content is wider than the viewport", () => {
    const viewport = computeFitViewport(500, 400, 1000, 200, 20);
    expect(viewport.scale).toBe(0.5);
    expect(viewport.offsetX).toBe(0);
  });
});
