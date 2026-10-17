import type {
  SSOAdminConfigVM,
  SSOProvider,
  UpdateSSOConfigCommand,
} from "@sparkops/contracts/identity";

export type SSOConfigDraft = {
  version: number;
  loginUI: {
    enabled: boolean;
    provider: SSOProvider;
    showPasswordLogin: boolean;
  };
  oidc: {
    enabled: boolean;
    issuer: string;
    clientId: string;
    clientSecret: string;
    hasClientSecret: boolean;
    clearClientSecret: boolean;
    redirectUrl: string;
    scopes: string;
    authorizationEndpoint: string;
    tokenEndpoint: string;
    userInfoEndpoint: string;
    testedFingerprint: string;
    testedAt?: string;
  };
  ldap: {
    enabled: boolean;
    url: string;
    bindDn: string;
    bindPassword: string;
    hasBindPassword: boolean;
    clearBindPassword: boolean;
    userSearchBase: string;
    userSearchFilter: string;
    emailAttribute: string;
    nameAttribute: string;
    testedFingerprint: string;
    testedAt?: string;
  };
};

type OIDCConnectionKey =
  | "issuer"
  | "clientId"
  | "clientSecret"
  | "clearClientSecret"
  | "redirectUrl"
  | "scopes"
  | "authorizationEndpoint"
  | "tokenEndpoint"
  | "userInfoEndpoint";

type LDAPConnectionKey =
  | "url"
  | "bindDn"
  | "bindPassword"
  | "clearBindPassword"
  | "userSearchBase"
  | "userSearchFilter"
  | "emailAttribute"
  | "nameAttribute";

const OIDC_CONNECTION_KEYS: readonly OIDCConnectionKey[] = [
  "issuer",
  "clientId",
  "clientSecret",
  "clearClientSecret",
  "redirectUrl",
  "scopes",
  "authorizationEndpoint",
  "tokenEndpoint",
  "userInfoEndpoint",
];

const LDAP_CONNECTION_KEYS: readonly LDAPConnectionKey[] = [
  "url",
  "bindDn",
  "bindPassword",
  "clearBindPassword",
  "userSearchBase",
  "userSearchFilter",
  "emailAttribute",
  "nameAttribute",
];

function patchChangesConnection<T extends object, K extends keyof T>(
  current: T,
  patch: Partial<T>,
  keys: readonly K[],
) {
  return keys.some(
    (key) =>
      Object.prototype.hasOwnProperty.call(patch, key) &&
      patch[key] !== current[key],
  );
}

export function createSSODraft(config: SSOAdminConfigVM): SSOConfigDraft {
  return {
    version: config.version,
    loginUI: {
      enabled: config.login_ui.enabled,
      provider: config.login_ui.provider,
      showPasswordLogin: config.login_ui.show_password_login,
    },
    oidc: {
      enabled: config.oidc.enabled,
      issuer: config.oidc.issuer,
      clientId: config.oidc.client_id,
      clientSecret: "",
      hasClientSecret: config.oidc.has_client_secret,
      clearClientSecret: false,
      redirectUrl: config.oidc.redirect_url,
      scopes: config.oidc.scopes,
      authorizationEndpoint: config.oidc.authorization_endpoint,
      tokenEndpoint: config.oidc.token_endpoint,
      userInfoEndpoint: config.oidc.userinfo_endpoint,
      testedFingerprint: config.oidc.tested_fingerprint,
      testedAt: config.oidc.tested_at,
    },
    ldap: {
      enabled: config.ldap.enabled,
      url: config.ldap.url,
      bindDn: config.ldap.bind_dn,
      bindPassword: "",
      hasBindPassword: config.ldap.has_bind_password,
      clearBindPassword: false,
      userSearchBase: config.ldap.user_search_base,
      userSearchFilter: config.ldap.user_search_filter,
      emailAttribute: config.ldap.email_attribute,
      nameAttribute: config.ldap.name_attribute,
      testedFingerprint: config.ldap.tested_fingerprint,
      testedAt: config.ldap.tested_at,
    },
  };
}

export function updateOIDCDraft(
  draft: SSOConfigDraft,
  patch: Partial<SSOConfigDraft["oidc"]>,
): SSOConfigDraft {
  const connectionChanged = patchChangesConnection(
    draft.oidc,
    patch,
    OIDC_CONNECTION_KEYS,
  );
  return {
    ...draft,
    oidc: {
      ...draft.oidc,
      ...patch,
      ...(connectionChanged
        ? { testedFingerprint: "" }
        : {}),
    },
  };
}

export function updateLDAPDraft(
  draft: SSOConfigDraft,
  patch: Partial<SSOConfigDraft["ldap"]>,
): SSOConfigDraft {
  const connectionChanged = patchChangesConnection(
    draft.ldap,
    patch,
    LDAP_CONNECTION_KEYS,
  );
  return {
    ...draft,
    ldap: {
      ...draft.ldap,
      ...patch,
      ...(connectionChanged
        ? { testedFingerprint: "" }
        : {}),
    },
  };
}

export function canEnableProvider(
  draft: SSOConfigDraft,
  provider: SSOProvider,
) {
  return Boolean(
    provider === "oidc"
      ? draft.oidc.testedFingerprint
      : draft.ldap.testedFingerprint,
  );
}

export function toUpdateCommand(
  draft: SSOConfigDraft,
): UpdateSSOConfigCommand {
  return {
    version: draft.version,
    login_ui: {
      enabled: draft.loginUI.enabled,
      provider: draft.loginUI.provider,
      show_password_login: draft.loginUI.showPasswordLogin,
    },
    oidc: {
      enabled: draft.oidc.enabled,
      issuer: draft.oidc.issuer,
      client_id: draft.oidc.clientId,
      client_secret: draft.oidc.clientSecret,
      clear_client_secret: draft.oidc.clearClientSecret,
      redirect_url: draft.oidc.redirectUrl,
      scopes: draft.oidc.scopes,
      authorization_endpoint: draft.oidc.authorizationEndpoint,
      token_endpoint: draft.oidc.tokenEndpoint,
      userinfo_endpoint: draft.oidc.userInfoEndpoint,
      tested_fingerprint: draft.oidc.testedFingerprint,
      tested_at: draft.oidc.testedFingerprint
        ? draft.oidc.testedAt
        : undefined,
    },
    ldap: {
      enabled: draft.ldap.enabled,
      url: draft.ldap.url,
      bind_dn: draft.ldap.bindDn,
      bind_password: draft.ldap.bindPassword,
      clear_bind_password: draft.ldap.clearBindPassword,
      user_search_base: draft.ldap.userSearchBase,
      user_search_filter: draft.ldap.userSearchFilter,
      email_attribute: draft.ldap.emailAttribute,
      name_attribute: draft.ldap.nameAttribute,
      tested_fingerprint: draft.ldap.testedFingerprint,
      tested_at: draft.ldap.testedFingerprint
        ? draft.ldap.testedAt
        : undefined,
    },
  };
}
