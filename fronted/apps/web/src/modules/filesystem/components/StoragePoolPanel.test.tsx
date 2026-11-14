import { describe, expect, it } from "vitest";
import { providerFixture } from "./storage-pool-fixtures";
import {
  canDeleteProvider,
  canShowStoragePoolMutations,
  createDefaultStoragePoolMembers,
  DEFAULT_LOCAL_PROVIDER_NAME,
  LOCAL_PROVIDER_DEFAULT_SECRET_REF,
  normalizeCreateStoragePoolCommand,
  normalizeRegisterProviderCommand,
  PROVIDER_CREDENTIAL_LABEL,
  providerRowsForDisplay,
  registerProviderSecretPlaceholder,
  rowTextWouldExposeCredentials,
  rowsExposeProviderSecretRefs,
  STORAGE_POOL_CREATE_LABEL,
  validateCreateStoragePoolForm,
  validateRegisterProviderForm,
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

  it("validates provider registration and defaults LOCAL secret ref", () => {
    expect(validateRegisterProviderForm({ name: " ", secret_ref: "", provider_type: "S3" })).toMatch(
      /名称/,
    );
    expect(validateRegisterProviderForm({ name: "p", secret_ref: "", provider_type: "S3" })).toMatch(
      /Secret/,
    );
    expect(validateRegisterProviderForm({ name: "p", secret_ref: "", provider_type: "LOCAL" })).toBeNull();

    const normalized = normalizeRegisterProviderCommand({
      name: " test ",
      provider_type: "LOCAL",
      secret_ref: "",
      endpoint: "  A:\\logs  ",
    });
    expect(normalized.name).toBe("test");
    expect(normalized.secret_ref).toBe(LOCAL_PROVIDER_DEFAULT_SECRET_REF);
    expect(normalized.endpoint).toBe("A:\\logs");
    expect(registerProviderSecretPlaceholder("LOCAL")).toContain(LOCAL_PROVIDER_DEFAULT_SECRET_REF);
  });

  it("creates a PRIMARY member from the first active provider", () => {
    const disabled = providerFixture({
      id: "33333333-3333-4333-8333-333333333333",
      name: "disabled",
      status: "DISABLED",
    });
    const active = providerFixture();

    expect(createDefaultStoragePoolMembers([disabled, active])).toEqual([
      {
        provider_id: active.id,
        mode: "PRIMARY",
        priority: 100,
      },
    ]);
  });

  it("validates storage pool members before submission", () => {
    const provider = providerFixture();
    const secondProvider = providerFixture({
      id: "33333333-3333-4333-8333-333333333333",
      name: "replica",
    });

    expect(validateCreateStoragePoolForm(" ", [], [provider])).toMatch(/名称/);
    expect(validateCreateStoragePoolForm("pool", [], [provider])).toMatch(/成员/);
    expect(
      validateCreateStoragePoolForm(
        "pool",
        [
          { provider_id: provider.id, mode: "PRIMARY", priority: 100 },
          { provider_id: provider.id, mode: "REPLICA", priority: 0 },
        ],
        [provider],
      ),
    ).toMatch(/重复/);
    expect(
      validateCreateStoragePoolForm(
        "pool",
        [{ provider_id: provider.id, mode: "REPLICA", priority: 0 }],
        [provider],
      ),
    ).toMatch(/PRIMARY/);
    expect(
      validateCreateStoragePoolForm(
        "pool",
        [
          { provider_id: provider.id, mode: "PRIMARY", priority: 100 },
          { provider_id: secondProvider.id, mode: "PRIMARY", priority: 100 },
        ],
        [provider, secondProvider],
      ),
    ).toMatch(/优先级/);
  });

  it("blocks deleting default local provider by name", () => {
    expect(canDeleteProvider({ name: DEFAULT_LOCAL_PROVIDER_NAME })).toBe(false);
    expect(canDeleteProvider({ name: "test-Local" })).toBe(true);
  });

  it("normalizes a storage pool command with members", () => {
    const command = normalizeCreateStoragePoolCommand("  production  ", [
      {
        provider_id: "11111111-1111-4111-8111-111111111111",
        mode: "PRIMARY",
        priority: 100,
      },
    ]);

    expect(command).toEqual({
      name: "production",
      members: [
        {
          provider_id: "11111111-1111-4111-8111-111111111111",
          mode: "PRIMARY",
          priority: 100,
        },
      ],
    });
  });
});
