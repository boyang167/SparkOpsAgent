import type { SSOAdminConfigVM } from "@sparkops/contracts/identity";
import { describe, expect, it } from "vitest";
import {
  canEnableProvider,
  createSSODraft,
  toUpdateCommand,
  updateLDAPDraft,
  updateOIDCDraft,
} from "./sso-config-state";

const config: SSOAdminConfigVM = {
  version: 7,
  login_ui: {
    enabled: true,
    provider: "oidc",
    show_password_login: true,
  },
  oidc: {
    enabled: true,
    issuer: "https://idp.example.com",
    client_id: "sparkops",
    redirect_url: "https://sparkops.example.com/api/auth/callback",
    scopes: "openid email profile",
    authorization_endpoint: "",
    token_endpoint: "",
    userinfo_endpoint: "",
    tested_fingerprint: "oidc-tested",
    tested_at: "2026-10-17T02:00:00Z",
    has_client_secret: true,
    runtime_enabled: true,
  },
  ldap: {
    enabled: true,
    url: "ldaps://ldap.example.com:636",
    bind_dn: "cn=reader,dc=example,dc=com",
    user_search_base: "ou=people,dc=example,dc=com",
    user_search_filter: "(uid=%s)",
    email_attribute: "mail",
    name_attribute: "cn",
    tested_fingerprint: "ldap-tested",
    tested_at: "2026-10-17T02:01:00Z",
    has_bind_password: true,
    runtime_enabled: true,
  },
};

describe("SSO configuration draft", () => {
  it("invalidates only OIDC test state when an OIDC connection field changes", () => {
    const draft = createSSODraft(config);
    const next = updateOIDCDraft(draft, { issuer: "https://new.example.com" });

    expect(next.oidc.testedFingerprint).toBe("");
    expect(next.ldap.testedFingerprint).toBe(draft.ldap.testedFingerprint);
  });

  it("invalidates only LDAP test state when an LDAP connection field changes", () => {
    const draft = createSSODraft(config);
    const next = updateLDAPDraft(draft, { userSearchBase: "ou=staff,dc=example,dc=com" });

    expect(next.ldap.testedFingerprint).toBe("");
    expect(next.oidc.testedFingerprint).toBe(draft.oidc.testedFingerprint);
  });

  it("does not invalidate a provider when only its enabled switch changes", () => {
    const draft = createSSODraft(config);

    expect(updateOIDCDraft(draft, { enabled: false }).oidc.testedFingerprint).toBe(
      draft.oidc.testedFingerprint,
    );
    expect(updateLDAPDraft(draft, { enabled: false }).ldap.testedFingerprint).toBe(
      draft.ldap.testedFingerprint,
    );
  });

  it("preserves existing secrets when password inputs remain empty", () => {
    const command = toUpdateCommand(createSSODraft(config));

    expect(command.oidc.client_secret).toBe("");
    expect(command.oidc.clear_client_secret).toBe(false);
    expect(command.ldap.bind_password).toBe("");
    expect(command.ldap.clear_bind_password).toBe(false);
  });

  it("sends explicit secret deletion and invalidates the corresponding test", () => {
    const oidcDraft = updateOIDCDraft(createSSODraft(config), {
      clearClientSecret: true,
    });
    const ldapDraft = updateLDAPDraft(createSSODraft(config), {
      clearBindPassword: true,
    });

    expect(oidcDraft.oidc.testedFingerprint).toBe("");
    expect(toUpdateCommand(oidcDraft).oidc.clear_client_secret).toBe(true);
    expect(ldapDraft.ldap.testedFingerprint).toBe("");
    expect(toUpdateCommand(ldapDraft).ldap.clear_bind_password).toBe(true);
  });

  it("requires a successful provider test before enabling it", () => {
    const draft = createSSODraft({
      ...config,
      oidc: { ...config.oidc, enabled: false, tested_fingerprint: "" },
    });

    expect(canEnableProvider(draft, "oidc")).toBe(false);
    expect(
      canEnableProvider(
        updateOIDCDraft(draft, { testedFingerprint: "new-test-result" }),
        "oidc",
      ),
    ).toBe(true);
  });

  it("does not copy read-only response fields into the update command", () => {
    const command = toUpdateCommand(createSSODraft(config));

    expect(command.oidc).not.toHaveProperty("has_client_secret");
    expect(command.oidc).not.toHaveProperty("runtime_enabled");
    expect(command.ldap).not.toHaveProperty("has_bind_password");
    expect(command.ldap).not.toHaveProperty("runtime_enabled");
  });
});
