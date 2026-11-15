import type {
  CreateStoragePoolCommand,
  RegisterStorageProviderCommand,
  StoragePoolMemberVM,
  StoragePoolVM,
  StorageProviderVM,
  UpdateStorageProviderCommand,
} from "@sparkops/contracts/filesystem";
import { canMutateStorage } from "./storage-admin-permissions";

export const LOCAL_PROVIDER_DEFAULT_SECRET_REF = "local://default";
export const DEFAULT_LOCAL_PROVIDER_NAME = "default-local";

export const PROVIDER_CREDENTIAL_LABEL = "已通过密钥引用配置";
export const STORAGE_POOL_CREATE_LABEL = "新增存储池";
export const STORAGE_PROVIDER_REGISTER_LABEL = "注册 Provider";

export type StoragePoolDraftMember = {
  provider_id: string;
  mode: StoragePoolMemberVM["mode"];
  priority: number;
};

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

export function validateRegisterProviderForm(
  form: Pick<RegisterStorageProviderCommand, "name" | "secret_ref" | "provider_type" | "endpoint">,
): string | null {
  if (!form.name.trim()) {
    return "请填写 Provider 名称";
  }
  if (!form.secret_ref.trim() && form.provider_type !== "LOCAL") {
    return "请填写 Secret 引用（仅提交引用路径，界面不会回显明文）";
  }
  if (!form.endpoint?.trim()) {
    return "请填写 Endpoint";
  }
  return null;
}

export function normalizeRegisterProviderCommand(
  form: RegisterStorageProviderCommand,
): RegisterStorageProviderCommand {
  const secretRef =
    form.secret_ref.trim() ||
    (form.provider_type === "LOCAL" ? LOCAL_PROVIDER_DEFAULT_SECRET_REF : "");
  return {
    ...form,
    name: form.name.trim(),
    secret_ref: secretRef,
    endpoint: form.endpoint?.trim() || undefined,
  };
}

export function registerProviderSecretPlaceholder(providerType: RegisterStorageProviderCommand["provider_type"]) {
  return providerType === "LOCAL"
    ? `${LOCAL_PROVIDER_DEFAULT_SECRET_REF}（留空则使用默认）`
    : "secret://...";
}

export function createDefaultStoragePoolMembers(
  providers: StorageProviderVM[],
): StoragePoolDraftMember[] {
  const provider = providers.find((item) => item.status === "ACTIVE");
  return provider
    ? [{ provider_id: provider.id, mode: "PRIMARY", priority: 100 }]
    : [];
}

export function validateCreateStoragePoolForm(
  name: string,
  members: StoragePoolDraftMember[],
  providers: StorageProviderVM[],
): string | null {
  if (!name.trim()) {
    return "请填写存储池名称";
  }
  if (members.length === 0) {
    return "请至少添加一个 Provider 成员";
  }
  const activeProviderIds = new Set(
    providers.filter((provider) => provider.status === "ACTIVE").map((provider) => provider.id),
  );
  if (members.some((member) => !activeProviderIds.has(member.provider_id))) {
    return "请选择可用的 Provider";
  }
  if (new Set(members.map((member) => member.provider_id)).size !== members.length) {
    return "同一个 Provider 不能重复添加";
  }
  const primaryMembers = members.filter((member) => member.mode === "PRIMARY");
  if (primaryMembers.length === 0) {
    return "存储池至少需要一个 PRIMARY 成员";
  }
  if (
    new Set(primaryMembers.map((member) => member.priority)).size !== primaryMembers.length
  ) {
    return "PRIMARY 成员的优先级不能重复";
  }
  return null;
}

export function normalizeCreateStoragePoolCommand(
  name: string,
  members: StoragePoolDraftMember[],
): CreateStoragePoolCommand {
  return {
    name: name.trim(),
    members: members.map((member) => ({
      provider_id: member.provider_id,
      mode: member.mode,
      priority: member.priority,
    })),
  };
}

export function canDeleteProvider(provider: Pick<StorageProviderVM, "name">) {
  return provider.name !== DEFAULT_LOCAL_PROVIDER_NAME;
}

export type ProviderHealthTone = "healthy" | "degraded" | "unknown";

export function providerHealthStatus(status: string): { label: string; tone: ProviderHealthTone } {
  const normalized = status.trim().toUpperCase();
  if (normalized === "ACTIVE") {
    return { label: "Healthy", tone: "healthy" };
  }
  if (normalized === "DISABLED") {
    return { label: "Degraded", tone: "degraded" };
  }
  return { label: status || "Unknown", tone: "unknown" };
}

export function providerKindSubtitle(provider: StorageProviderVM): string {
  switch (provider.provider_type) {
    case "S3":
      return provider.endpoint?.trim() ? `Amazon S3 · ${provider.endpoint.trim()}` : "Amazon S3";
    case "NFS":
      return provider.endpoint?.trim() ? `NFS · ${provider.endpoint.trim()}` : "NFS";
    case "LOCAL":
      return "Local Storage";
    default:
      return provider.provider_type;
  }
}

export function formatProviderCapabilities(provider: StorageProviderVM): string {
  const caps = provider.capabilities;
  if (caps && typeof caps === "object" && !Array.isArray(caps)) {
    const enabled = Object.entries(caps)
      .filter(([, value]) => value === true || value === "true")
      .map(([key]) => key);
    if (enabled.length > 0) {
      return `Capabilities: ${enabled.join(", ")}`;
    }
    const keys = Object.keys(caps);
    if (keys.length > 0) {
      return `Capabilities: ${keys.join(", ")}`;
    }
  }
  switch (provider.provider_type) {
    case "S3":
      return "Capabilities: Multipart, Versioning, Range Read";
    case "NFS":
      return "Capabilities: Mount, Range Read";
    case "LOCAL":
      return "Capabilities: Directory, Range Read";
    default:
      return "Capabilities: —";
  }
}

export function boundPoolNamesForProvider(providerId: string, pools: StoragePoolVM[]): string[] {
  return pools
    .filter((pool) => pool.members.some((member) => member.provider_id === providerId))
    .map((pool) => pool.name);
}

export function validateUpdateProviderForm(
  form: UpdateStorageProviderCommand,
  providerType?: RegisterStorageProviderCommand["provider_type"],
): string | null {
  if (!form.name.trim()) {
    return "请填写 Provider 名称";
  }
  if (providerType && !form.endpoint?.trim()) {
    return "请填写 Endpoint";
  }
  return null;
}

export function normalizeUpdateProviderCommand(
  form: UpdateStorageProviderCommand,
): UpdateStorageProviderCommand {
  const secretRef = form.secret_ref?.trim();
  return {
    name: form.name.trim(),
    endpoint: form.endpoint?.trim() || undefined,
    status: form.status?.trim() || "ACTIVE",
    ...(secretRef ? { secret_ref: secretRef } : {}),
  };
}

export function membersFromPool(pool: StoragePoolVM): StoragePoolDraftMember[] {
  return pool.members.map((member) => ({
    provider_id: member.provider_id,
    mode: member.mode,
    priority: member.priority,
  }));
}
