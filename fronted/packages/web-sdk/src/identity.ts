import type { AuthSessionVM, LoginCommand, MeVM, RegisterCommand } from "@sparkops/contracts/identity";
import { bffFetch } from "./client";

export function login(input: LoginCommand) {
  return bffFetch<AuthSessionVM>("/auth/login", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function register(input: RegisterCommand) {
  return bffFetch<AuthSessionVM>("/auth/register", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getSession() {
  return bffFetch<MeVM | null>("/auth/me");
}

export function logout() {
  return bffFetch<{ ok: true }>("/auth/logout", { method: "POST" });
}

export function getAuthProviders() {
  return bffFetch<import("@sparkops/contracts/identity").AuthProvidersVM>("/auth/providers");
}

export function startOIDCLogin() {
  return bffFetch<{ authorization_url: string; state: string }>("/auth/oidc/login");
}

export function completeOIDCCallback(code: string, state: string) {
  const qs = new URLSearchParams({ code, state });
  return bffFetch<{ memberId: string; email: string; displayName?: string }>(
    `/auth/oidc/callback?${qs}`,
  );
}

export function ldapLogin(input: { username: string; password: string }) {
  return bffFetch<{ memberId: string; email: string; displayName?: string }>("/auth/ldap/login", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getSSOAdminConfig() {
  return bffFetch<import("@sparkops/contracts/identity").SSOAdminConfigVM>("/admin/sso");
}

export function testSSOProvider(
  input: import("@sparkops/contracts/identity").TestSSOProviderCommand,
) {
  return bffFetch<import("@sparkops/contracts/identity").TestSSOProviderResultVM>(
    "/admin/sso/test",
    {
      method: "POST",
      body: JSON.stringify(input),
    },
  );
}

export function updateSSOAdminConfig(
  input: import("@sparkops/contracts/identity").UpdateSSOConfigCommand,
) {
  return bffFetch<import("@sparkops/contracts/identity").SSOAdminConfigVM>("/admin/sso", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function listPlatformMembers() {
  return bffFetch<{ items: import("@sparkops/contracts/identity").PlatformMemberVM[] }>(
    "/platform/members",
  ).then((data) => data.items);
}

export function createPlatformMember(input: import("@sparkops/contracts/identity").CreatePlatformMemberCommand) {
  return bffFetch<{ id: string; email: string; is_super_admin: boolean }>("/platform/members", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updatePlatformMember(
  memberId: string,
  input: import("@sparkops/contracts/identity").UpdatePlatformMemberCommand,
) {
  return bffFetch<{ updated: true }>(`/platform/members/${memberId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function resetPlatformMemberPassword(
  memberId: string,
  input: import("@sparkops/contracts/identity").AdminResetPasswordCommand,
) {
  return bffFetch<{ updated: true }>(`/platform/members/${memberId}/password`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deletePlatformMember(memberId: string) {
  return bffFetch<{ deleted: true }>(`/platform/members/${memberId}`, { method: "DELETE" });
}

export { updateProfile, changePassword } from "./profile";
