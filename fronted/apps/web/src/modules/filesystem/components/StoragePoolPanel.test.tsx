import { describe, expect, it } from "vitest";
import { providerFixture } from "./storage-pool-fixtures";
import {
  canShowStoragePoolMutations,
  PROVIDER_CREDENTIAL_LABEL,
  providerRowsForDisplay,
  rowTextWouldExposeCredentials,
  rowsExposeProviderSecretRefs,
  STORAGE_POOL_CREATE_LABEL,
  visibleMutationButtons,
} from "./storage-pool-panel-view";

/**
 * Vitest cannot transform Next `jsx: preserve` panels in this toolchain (see FileExplorerPanel.test.ts).
 */
describe("StoragePoolPanel", () => {
  it("never renders provider credentials and hides mutations without storage admin", async () => {
    const provider = providerFixture();
    const rows = providerRowsForDisplay([provider]);
    expect(rowsExposeProviderSecretRefs([provider], rows)).toBe(false);
    expect(rowTextWouldExposeCredentials(rows)).toBe(false);
    expect(canShowStoragePoolMutations(["storage:read"])).toBe(false);
    expect(visibleMutationButtons(["storage:read"])).not.toContain(STORAGE_POOL_CREATE_LABEL);
    expect(rows[0]?.credential).toBe(PROVIDER_CREDENTIAL_LABEL);
    expect(rows[0]?.name).toBe("primary-s3");
  });

  it("allows storage pool mutations when storage admin is granted", () => {
    expect(canShowStoragePoolMutations(["storage:read", "storage:admin"])).toBe(true);
    expect(visibleMutationButtons(["storage:read", "storage:admin"])).toContain(
      STORAGE_POOL_CREATE_LABEL,
    );
  });
});
