import { describe, expect, it } from "vitest";
import {
  SSOAdminConfigVMSchema,
  TestSSOProviderCommandSchema,
} from "./index";

const config = {
  version: 1,
  login_ui: {
    enabled: true,
    provider: "oidc",
    show_password_login: true,
  },
  oidc: {
    enabled: true,
    issuer: "https://idp.example.com",
    client_id: "sparkops",
    redirect_url: "https://sparkops.example.com/callback",
    scopes: "openid email profile",
    authorization_endpoint: "",
    token_endpoint: "",
    userinfo_endpoint: "",
    tested_fingerprint: "fingerprint",
    tested_at: "2026-10-17T02:00:00Z",
    has_client_secret: true,
    runtime_enabled: true,
  },
  ldap: {
    enabled: false,
    url: "",
    bind_dn: "",
    user_search_base: "",
    user_search_filter: "(uid=%s)",
    email_attribute: "mail",
    name_attribute: "cn",
    tested_fingerprint: "",
    has_bind_password: false,
    runtime_enabled: false,
  },
};

describe("SSO administration contracts", () => {
  it("accepts a masked administration response", () => {
    expect(SSOAdminConfigVMSchema.parse(config)).toEqual(config);
  });

  it("rejects secrets in a read response", () => {
    expect(
      SSOAdminConfigVMSchema.safeParse({
        ...config,
        oidc: { ...config.oidc, client_secret: "must-not-leak" },
      }).success,
    ).toBe(false);
    expect(
      SSOAdminConfigVMSchema.safeParse({
        ...config,
        ldap: { ...config.ldap, bind_password: "must-not-leak" },
      }).success,
    ).toBe(false);
  });

  it("allows secrets only in test and update commands", () => {
    const {
      has_client_secret: _hasClientSecret,
      runtime_enabled: _oidcRuntimeEnabled,
      ...oidc
    } = config.oidc;
    const {
      has_bind_password: _hasBindPassword,
      runtime_enabled: _ldapRuntimeEnabled,
      ...ldap
    } = config.ldap;
    const result = TestSSOProviderCommandSchema.parse({
      provider: "oidc",
      config: {
        ...config,
        oidc: {
          ...oidc,
          client_secret: "new-secret",
          clear_client_secret: false,
        },
        ldap: {
          ...ldap,
          bind_password: "",
          clear_bind_password: false,
        },
      },
    });
    expect(result.provider).toBe("oidc");
    expect(result.config.oidc.client_secret).toBe("new-secret");
  });
});
