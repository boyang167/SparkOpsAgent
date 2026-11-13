import type { StorageProviderVM } from "@sparkops/contracts/filesystem";
import { canMutateStorage } from "./storage-admin-permissions";

export const PROVIDER_CREDENTIAL_LABEL = "已通过密钥引用配置";
export const STORAGE_POOL_CREATE_LABEL = "新增存储池";
export const STORAGE_PROVIDER_REGISTER_LABEL = "注册 Provider";

export type ProviderTableRow = {
  id: string;
  name: string;
  provider_type: string;
  endpoint: string;
  credential: string;
  status: string;
};

export function canShowStoragePoolMutations(permissions: string[]) {
  return canMutateStorage(permissions);
}

export function buildProviderTableRow(provider: StorageProviderVM): ProviderTableRow {
  return {
    id: provider.id,
    name: provider.name,
    provider_type: provider.provider_type,
    endpoint: provider.endpoint || "—",
    credential: PROVIDER_CREDENTIAL_LABEL,
    status: provider.status,
  };
}

export function providerRowsForDisplay(providers: StorageProviderVM[]) {
  return providers.map(buildProviderTableRow);
}

export function rowsExposeProviderSecretRefs(
  providers: StorageProviderVM[],
  rows: ProviderTableRow[],
) {
  const rowPayload = JSON.stringify(rows);
  return providers.some((provider) => rowPayload.includes(provider.secret_ref));
}

export function rowTextWouldExposeCredentials(rows: ProviderTableRow[]) {
  const rowPayload = JSON.stringify(rows);
  return /secret|access.?key/i.test(rowPayload);
}

export function visibleMutationButtons(permissions: string[]) {
  if (!canShowStoragePoolMutations(permissions)) {
    return [];
  }
  return [STORAGE_PROVIDER_REGISTER_LABEL, STORAGE_POOL_CREATE_LABEL];
}
