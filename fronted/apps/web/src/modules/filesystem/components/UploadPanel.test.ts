import { describe, expect, it } from "vitest";
import { getUploadGate, UPLOAD_DISABLED_HINT } from "./upload-panel-gate";

describe("UploadPanel", () => {
  it("disables upload when providerId is missing", () => {
    const gate = getUploadGate(undefined);
    expect(gate.uploadEnabled).toBe(false);
    expect(gate.helperText).toBe(UPLOAD_DISABLED_HINT);
  });

  it("enables upload when providerId is present", () => {
    const gate = getUploadGate("provider-1");
    expect(gate.uploadEnabled).toBe(true);
    expect(gate.helperText).toBeNull();
  });
});
