import { bffFetch } from "./client";
import type { CreateSecretCommand, SecretVM, UpdateSecretCommand } from "@sparkops/contracts/secret";

export async function listSecrets(workspaceId: string) {
  const data = await bffFetch<{ items: SecretVM[] }>(`/workspaces/${workspaceId}/secrets`, {
    workspaceId,
  });
  return data.items;
}

export function getSecret(workspaceId: string, secretId: string) {
  return bffFetch<SecretVM>(`/workspaces/${workspaceId}/secrets/${secretId}`, { workspaceId });
}

export function createSecret(workspaceId: string, input: CreateSecretCommand) {
  return bffFetch<SecretVM>(`/workspaces/${workspaceId}/secrets`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function updateSecret(workspaceId: string, secretId: string, input: UpdateSecretCommand) {
  return bffFetch<SecretVM>(`/workspaces/${workspaceId}/secrets/${secretId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function deleteSecret(workspaceId: string, secretId: string) {
  return bffFetch<{ deleted: boolean }>(`/workspaces/${workspaceId}/secrets/${secretId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export function testSecretConnection(workspaceId: string, secretId: string) {
  return bffFetch<{ success: boolean; message: string }>(
    `/workspaces/${workspaceId}/secrets/${secretId}/test`,
    { method: "POST", workspaceId },
  );
}
