import { describe, expect, it } from "vitest";
import { hasVersionPlaceholder, runtimeVersionTag } from "./create-version-job-spec";

describe("runtimeVersionTag", () => {
  it("drops unresolved placeholders so the run form requires a real tag", () => {
    expect(runtimeVersionTag("{{version}}")).toBe("");
    expect(runtimeVersionTag("v{{.version}}")).toBe("");
    expect(hasVersionPlaceholder("{{version}}")).toBe(true);
  });

  it("keeps concrete tags", () => {
    expect(runtimeVersionTag("  v1.0.0 ")).toBe("v1.0.0");
    expect(hasVersionPlaceholder("v1.0.0")).toBe(false);
  });
});
