import type { BffContext } from "@sparkops/bff-core";

export async function listSecrets(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.secret.listSecrets(workspaceId);
  return data.items;
}

export async function getSecret(ctx: BffContext, workspaceId: string, secretId: string) {
  return ctx.upstream.secret.getSecret(workspaceId, secretId);
}

export async function createSecret(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/secret").CreateSecretCommand,
) {
  return ctx.upstream.secret.createSecret(workspaceId, input);
}

export async function updateSecret(
  ctx: BffContext,
  workspaceId: string,
  secretId: string,
  input: import("@sparkops/contracts/secret").UpdateSecretCommand,
) {
  return ctx.upstream.secret.updateSecret(workspaceId, secretId, input);
}

export async function deleteSecret(ctx: BffContext, workspaceId: string, secretId: string) {
  return ctx.upstream.secret.deleteSecret(workspaceId, secretId);
}

export async function testSecretConnection(ctx: BffContext, workspaceId: string, secretId: string) {
  return ctx.upstream.secret.testSecretConnection(workspaceId, secretId);
}
