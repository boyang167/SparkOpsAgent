import type { StorageProviderVM } from "@sparkops/contracts/filesystem";

export function providerFixture(overrides: Partial<StorageProviderVM> = {}): StorageProviderVM {
  return {
    id: "11111111-1111-4111-8111-111111111111",
    organization_id: "22222222-2222-4222-8222-222222222222",
    name: "primary-s3",
    provider_type: "S3",
    endpoint: "https://s3.example.com",
    secret_ref: "secret://filesystem/providers/access-key",
    capabilities: {},
    config_version: 1,
    status: "ACTIVE",
    ...overrides,
  };
}
