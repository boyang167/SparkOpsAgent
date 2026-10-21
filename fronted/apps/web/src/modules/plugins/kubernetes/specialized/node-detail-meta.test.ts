import { describe, expect, it } from "vitest";
import { formatMetaKey, isSystemMetaKey, splitMetaEntries } from "./node-detail-meta";

describe("node-detail-meta", () => {
  it("splits system and user labels", () => {
    const result = splitMetaEntries({
      "node-role.kubernetes.io/master": "",
      "app.kubernetes.io/name": "demo",
      "projectcalico.org/ipv4address": "10.0.0.1/24",
    });
    expect(result.user).toEqual([["app.kubernetes.io/name", "demo"]]);
    expect(result.system.map(([key]) => key)).toEqual([
      "node-role.kubernetes.io/master",
      "projectcalico.org/ipv4address",
    ]);
  });

  it("shortens role label keys for display", () => {
    expect(formatMetaKey("node-role.kubernetes.io/control-plane")).toBe("control-plane");
    expect(formatMetaKey("custom/owner")).toBe("owner");
  });

  it("detects kubernetes system prefixes", () => {
    expect(isSystemMetaKey("kubernetes.io/arch")).toBe(true);
    expect(isSystemMetaKey("team")).toBe(false);
  });
});
