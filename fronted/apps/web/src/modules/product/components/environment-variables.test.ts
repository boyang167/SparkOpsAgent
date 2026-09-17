import { describe, expect, it } from "vitest";
import { rowsToVariables, variablesToRows } from "./environment-variables";

describe("environment variables helpers", () => {
  it("keeps an empty editor row when no variables exist", () => {
    expect(variablesToRows()).toEqual([{ key: "", value: "" }]);
  });

  it("rejects duplicate keys", () => {
    const result = rowsToVariables([
      { key: "domain", value: "a.com" },
      { key: "domain", value: "b.com" },
    ]);
    expect(result.error).toContain("重复");
  });

  it("skips blank rows and keeps values", () => {
    const result = rowsToVariables([
      { key: " domain ", value: "vote-dev.example.com" },
      { key: "", value: "" },
    ]);
    expect(result.error).toBeNull();
    expect(result.variables).toEqual({ domain: "vote-dev.example.com" });
  });
});
