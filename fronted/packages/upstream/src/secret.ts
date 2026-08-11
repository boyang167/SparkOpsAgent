import type { CreateSecretCommand, SecretVM, UpdateSecretCommand } from "@sparkops/contracts/secret";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createSecretUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listSecrets(workspaceId: string) {
      return upstreamFetch<{ items: SecretVM[] }>(
        `/api/v1/workspaces/${workspaceId}/secrets`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getSecret(workspaceId: string, secretId: string) {
      return upstreamFetch<SecretVM>(
        `/api/v1/workspaces/${workspaceId}/secrets/${secretId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createSecret(workspaceId: string, input: CreateSecretCommand) {
      return upstreamFetch<SecretVM>(
        `/api/v1/workspaces/${workspaceId}/secrets`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateSecret(workspaceId: string, secretId: string, input: UpdateSecretCommand) {
      return upstreamFetch<SecretVM>(
        `/api/v1/workspaces/${workspaceId}/secrets/${secretId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteSecret(workspaceId: string, secretId: string) {
      return upstreamFetch<{ deleted: boolean }>(
        `/api/v1/workspaces/${workspaceId}/secrets/${secretId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async testSecretConnection(workspaceId: string, secretId: string) {
      return upstreamFetch<{ success: boolean; message: string }>(
        `/api/v1/workspaces/${workspaceId}/secrets/${secretId}/test`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },
  };
}

export type SecretUpstream = ReturnType<typeof createSecretUpstream>;
