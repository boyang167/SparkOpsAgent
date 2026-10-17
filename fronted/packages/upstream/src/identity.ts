import type {
  LoginCommand,
  MeVM,
  RegisterCommand,
  SSOAdminConfigVM,
  TestSSOProviderCommand,
  TestSSOProviderResultVM,
  UpdateSSOConfigCommand,
} from "@sparkops/contracts/identity";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

type LoginUpstreamData = {
  token: string;
  id?: string;
  email?: string;
};

type RegisterUpstreamData = {
  id: string;
  email: string;
};

type MeUpstreamData = {
  id: string;
  email: string;
  display_name: string;
  is_super_admin?: boolean;
};

export function createIdentityUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async login(input: LoginCommand) {
      const data = await upstreamFetch<LoginUpstreamData>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email: input.email, password: input.password }),
        options,
      });
      return data;
    },

    async register(input: RegisterCommand) {
      return upstreamFetch<RegisterUpstreamData>("/api/v1/auth/register", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },

    async me(token: string) {
      return upstreamFetch<MeUpstreamData>("/api/v1/auth/me", {
        method: "GET",
        options: { ...options, token },
      });
    },

    async updateProfile(token: string, input: { display_name: string }) {
      return upstreamFetch<MeUpstreamData>("/api/v1/auth/me", {
        method: "PATCH",
        body: JSON.stringify(input),
        options: { ...options, token },
      });
    },

    async changePassword(token: string, input: { current_password: string; new_password: string }) {
      return upstreamFetch<{ changed: true }>("/api/v1/auth/password", {
        method: "PATCH",
        body: JSON.stringify(input),
        options: { ...options, token },
      });
    },

    async getAuthProviders() {
      return upstreamFetch<{ oidc_enabled: boolean; ldap_enabled: boolean }>("/api/v1/auth/providers", {
        method: "GET",
        options,
      });
    },

    async startOIDCLogin() {
      return upstreamFetch<{ authorization_url: string; state: string }>("/api/v1/auth/oidc/login", {
        method: "GET",
        options,
      });
    },

    async completeOIDCCallback(code: string, state: string) {
      const qs = new URLSearchParams({ code, state });
      return upstreamFetch<LoginUpstreamData>(`/api/v1/auth/oidc/callback?${qs}`, {
        method: "GET",
        options,
      });
    },

    async ldapLogin(input: { username: string; password: string }) {
      return upstreamFetch<LoginUpstreamData>("/api/v1/auth/ldap/login", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },

    async getSSOAdminConfig() {
      return upstreamFetch<SSOAdminConfigVM>("/api/v1/admin/sso", {
        method: "GET",
        options,
      });
    },

    async testSSOProvider(input: TestSSOProviderCommand) {
      return upstreamFetch<TestSSOProviderResultVM>("/api/v1/admin/sso/test", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },

    async updateSSOAdminConfig(input: UpdateSSOConfigCommand) {
      return upstreamFetch<SSOAdminConfigVM>("/api/v1/admin/sso", {
        method: "PUT",
        body: JSON.stringify(input),
        options,
      });
    },

    async listPlatformMembers() {
      return upstreamFetch<{ items: import("@sparkops/contracts/identity").PlatformMemberVM[] }>(
        "/api/v1/platform/members",
        { method: "GET", options },
      );
    },
    async createPlatformMember(input: import("@sparkops/contracts/identity").CreatePlatformMemberCommand) {
      return upstreamFetch<{ id: string; email: string; is_super_admin: boolean }>(
        "/api/v1/platform/members",
        { method: "POST", body: JSON.stringify(input), options },
      );
    },
    async updatePlatformMember(
      memberId: string,
      input: import("@sparkops/contracts/identity").UpdatePlatformMemberCommand,
    ) {
      return upstreamFetch<{ updated: true }>(`/api/v1/platform/members/${memberId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async resetPlatformMemberPassword(
      memberId: string,
      input: import("@sparkops/contracts/identity").AdminResetPasswordCommand,
    ) {
      return upstreamFetch<{ updated: true }>(`/api/v1/platform/members/${memberId}/password`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },
    async deletePlatformMember(memberId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/platform/members/${memberId}`, {
        method: "DELETE",
        options,
      });
    },
  };
}

export type IdentityUpstream = ReturnType<typeof createIdentityUpstream>;
