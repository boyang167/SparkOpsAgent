"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  FEATURE_MODULE_LABELS,
  type WorkspaceEntitlementsVM,
} from "@sparkops/contracts/platform";
import type { AuthProvidersVM } from "@sparkops/contracts/identity";
import { identity, platform, tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";

type FeatureKey = keyof WorkspaceEntitlementsVM;

const FEATURE_KEYS = Object.keys(FEATURE_MODULE_LABELS) as FeatureKey[];

const LICENSE_EDITIONS = [
  { value: "community", label: "Community" },
  { value: "professional", label: "Professional" },
  { value: "enterprise", label: "Enterprise" },
] as const;

type WorkspaceFeatureOverrides = Record<string, Partial<WorkspaceEntitlementsVM>>;

function FeatureSwitch({
  label,
  description,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  description?: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (next: boolean) => void;
}) {
  return (
    <div className="config-switch-row">
      <div>
        <div>{label}</div>
        {description ? <div className="text-muted text-xs mt-1">{description}</div> : null}
      </div>
      <button
        type="button"
        className={checked ? "menu-switch menu-switch-on" : "menu-switch"}
        aria-pressed={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
      >
        <span className="menu-switch-thumb" />
      </button>
    </div>
  );
}

function readFeatures(config: Record<string, unknown> | undefined): WorkspaceEntitlementsVM {
  const defaults: WorkspaceEntitlementsVM = {
    agent: true,
    knowledge: true,
    marketplace: true,
    delivery: true,
    observability: true,
    k8s: false,
    mcp: false,
  };
  const raw = config?.features;
  if (!raw || typeof raw !== "object") return defaults;
  const features = raw as Record<string, unknown>;
  for (const key of FEATURE_KEYS) {
    if (typeof features[key] === "boolean") {
      defaults[key] = features[key] as boolean;
    }
  }
  return defaults;
}

function readWorkspaceFeatures(config: Record<string, unknown> | undefined): WorkspaceFeatureOverrides {
  const raw = config?.workspace_features;
  if (!raw || typeof raw !== "object") return {};
  const out: WorkspaceFeatureOverrides = {};
  for (const [wsId, value] of Object.entries(raw as Record<string, unknown>)) {
    if (!value || typeof value !== "object") continue;
    const parsed: Partial<WorkspaceEntitlementsVM> = {};
    for (const key of FEATURE_KEYS) {
      const item = (value as Record<string, unknown>)[key];
      if (typeof item === "boolean") {
        parsed[key] = item;
      }
    }
    if (Object.keys(parsed).length > 0) {
      out[wsId] = parsed;
    }
  }
  return out;
}

function isLicenseExpired(expiresAt: string | null): boolean {
  if (!expiresAt) return false;
  const date = new Date(expiresAt);
  return !Number.isNaN(date.getTime()) && date.getTime() < Date.now();
}

export function PlatformSettingsPanel({ variant = "default" }: { variant?: "platform" | "default" }) {
  const isPlatform = variant === "platform";
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [features, setFeatures] = useState<WorkspaceEntitlementsVM>(() => readFeatures(undefined));
  const [providers, setProviders] = useState<AuthProvidersVM | null>(null);
  const [workspaces, setWorkspaces] = useState<Array<{ id: string; name: string }>>([]);
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState("");
  const [workspaceFeatures, setWorkspaceFeatures] = useState<WorkspaceFeatureOverrides>({});

  const [ssoEnabled, setSsoEnabled] = useState(false);
  const [ssoProvider, setSsoProvider] = useState("oidc");
  const [showPasswordLogin, setShowPasswordLogin] = useState(true);

  const [licenseEdition, setLicenseEdition] = useState("community");
  const [licenseExpiresAt, setLicenseExpiresAt] = useState("");

  const licenseExpired = useMemo(() => isLicenseExpired(licenseExpiresAt || null), [licenseExpiresAt]);

  const selectedWorkspaceOverrides = useMemo(() => {
    if (!selectedWorkspaceId) {
      return {
        agent: true,
        knowledge: true,
        marketplace: true,
        delivery: true,
        observability: true,
        k8s: true,
        mcp: true,
      } satisfies WorkspaceEntitlementsVM;
    }
    const base = readFeatures(config);
    const overrides = workspaceFeatures[selectedWorkspaceId] ?? {};
    return { ...base, ...overrides };
  }, [selectedWorkspaceId, workspaceFeatures, config]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [data, authProviders, workspaceList] = await Promise.all([
        platform.getSysConfig(),
        identity.getAuthProviders().catch(() => null),
        tenant.listWorkspaces().catch(() => []),
      ]);
      const nextConfig = (data.config ?? {}) as Record<string, unknown>;
      setConfig(nextConfig);
      setFeatures(readFeatures(nextConfig));
      setWorkspaceFeatures(readWorkspaceFeatures(nextConfig));
      setProviders(authProviders);
      setWorkspaces(workspaceList.map((item) => ({ id: item.id, name: item.name })));
      setSelectedWorkspaceId((current) => current || workspaceList[0]?.id || "");

      const sso = nextConfig.sso;
      if (sso && typeof sso === "object") {
        const item = sso as Record<string, unknown>;
        setSsoEnabled(item.enabled === true);
        setSsoProvider(typeof item.provider === "string" ? item.provider : "oidc");
        setShowPasswordLogin(item.show_password_login !== false);
      }

      const license = nextConfig.license;
      if (license && typeof license === "object") {
        const item = license as Record<string, unknown>;
        setLicenseEdition(typeof item.edition === "string" ? item.edition : "community");
        const expires = typeof item.expires_at === "string" ? item.expires_at : "";
        setLicenseExpiresAt(expires.length >= 10 ? expires.slice(0, 10) : expires);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载平台配置失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function persistConfig(nextConfig: Record<string, unknown>, successMessage: string) {
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      await platform.updateSysConfig({ config: nextConfig });
      setConfig(nextConfig);
      setMessage(successMessage);
      notifyEntitlementsUpdated();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function onSaveFeatures() {
    const nextConfig = {
      ...config,
      features: { ...features },
      sso: buildSsoConfig(),
      license: buildLicenseConfig(),
      workspace_features: workspaceFeatures,
    };
    await persistConfig(nextConfig, "功能模块配置已保存，菜单将按新能力过滤。");
  }

  async function onSaveSso() {
    const nextConfig = {
      ...config,
      features: config.features ?? { ...features },
      sso: buildSsoConfig(),
      license: buildLicenseConfig(),
      workspace_features: workspaceFeatures,
    };
    await persistConfig(nextConfig, "SSO 登录策略已保存。");
  }

  async function onSaveLicense() {
    const nextConfig = {
      ...config,
      features: config.features ?? { ...features },
      sso: buildSsoConfig(),
      license: buildLicenseConfig(),
      workspace_features: workspaceFeatures,
    };
    await persistConfig(
      nextConfig,
      licenseExpired ? "许可证已保存（当前已过期，部分模块将被限制）。" : "许可证已保存。",
    );
  }

  async function onSaveWorkspaceOverrides() {
    if (!selectedWorkspaceId) return;
    const nextConfig = {
      ...config,
      features: config.features ?? { ...features },
      sso: buildSsoConfig(),
      license: buildLicenseConfig(),
      workspace_features: workspaceFeatures,
    };
    await persistConfig(nextConfig, "工作区能力覆盖已保存。");
  }

  function buildSsoConfig() {
    return {
      enabled: ssoEnabled,
      provider: ssoProvider,
      show_password_login: showPasswordLogin,
    };
  }

  function buildLicenseConfig() {
    return {
      edition: licenseEdition,
      expires_at: licenseExpiresAt ? `${licenseExpiresAt}T23:59:59Z` : null,
    };
  }

  function updateWorkspaceOverride(key: FeatureKey, value: boolean) {
    if (!selectedWorkspaceId) return;
    setWorkspaceFeatures((prev) => ({
      ...prev,
      [selectedWorkspaceId]: {
        ...(prev[selectedWorkspaceId] ?? {}),
        [key]: value,
      },
    }));
  }

  if (loading) {
    return <div className="empty-state">加载平台配置…</div>;
  }

  const sections = (
    <>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {message ? <InlineAlert tone="success">{message}</InlineAlert> : null}

      <section className="card stack-md">
        <div>
          <h2 className="text-lg font-semibold">功能模块</h2>
          <p className="text-muted text-sm mt-1">
            控制业务控制台菜单与 Cmd+K 可见性。K8s 模块在未接入集群时默认关闭；接入集群后会自动启用（除非在此显式关闭）。
          </p>
        </div>
        <div className="stack-sm">
          {FEATURE_KEYS.map((key) => (
            <FeatureSwitch
              key={key}
              label={FEATURE_MODULE_LABELS[key]}
              description={
                key === "k8s"
                  ? "运行实例、定时任务、基础设施等依赖 Kubernetes 的能力"
                  : key === "mcp"
                    ? "外部集成 / Webhook / MCP 工具链"
                    : undefined
              }
              checked={features[key]}
              onChange={(next) => setFeatures((prev) => ({ ...prev, [key]: next }))}
            />
          ))}
        </div>
        <div className="flex justify-end">
          <button type="button" className="btn" disabled={saving} onClick={() => void onSaveFeatures()}>
            {saving ? "保存中…" : "保存模块配置"}
          </button>
        </div>
      </section>

      <section className="card stack-md">
        <div>
          <h2 className="text-lg font-semibold">单点登录 (SSO)</h2>
          <p className="text-muted text-sm mt-1">
            运行时 OIDC/LDAP 由服务端 config.yaml 控制；此处配置登录页展示策略。
          </p>
        </div>
        <div className="stack-sm text-sm">
          <div className="config-row">
            <span>OIDC 运行时</span>
            <span className="text-muted">{providers?.oidc_enabled ? "已启用" : "未配置"}</span>
          </div>
          <div className="config-row">
            <span>LDAP 运行时</span>
            <span className="text-muted">{providers?.ldap_enabled ? "已启用" : "未配置"}</span>
          </div>
          <FeatureSwitch
            label="登录页展示企业 SSO"
            description="关闭后登录页仅显示邮箱密码（若允许）"
            checked={ssoEnabled}
            onChange={setSsoEnabled}
          />
          <div className="config-row">
            <label htmlFor="sso-provider">默认 SSO 提供商</label>
            <select
              id="sso-provider"
              className="input max-w-xs"
              value={ssoProvider}
              onChange={(e) => setSsoProvider(e.target.value)}
            >
              <option value="oidc">OIDC</option>
              <option value="ldap">LDAP</option>
            </select>
          </div>
          <FeatureSwitch
            label="允许邮箱密码登录"
            checked={showPasswordLogin}
            onChange={setShowPasswordLogin}
          />
        </div>
        <div className="flex justify-end">
          <button type="button" className="btn" disabled={saving} onClick={() => void onSaveSso()}>
            {saving ? "保存中…" : "保存 SSO 配置"}
          </button>
        </div>
      </section>

      <section className="card stack-md">
        <div>
          <h2 className="text-lg font-semibold">许可证</h2>
          <p className="text-muted text-sm mt-1">
            版本决定 MCP 等高级模块上限；到期后将降级为基础能力（智能体 + 知识库）。
          </p>
        </div>
        {licenseExpired ? (
          <InlineAlert tone="error">许可证已过期，交付/K8s/可观测等模块将被自动禁用。</InlineAlert>
        ) : null}
        <div className="stack-sm text-sm">
          <div className="config-row">
            <label htmlFor="license-edition">版本</label>
            <select
              id="license-edition"
              className="input max-w-xs"
              value={licenseEdition}
              onChange={(e) => setLicenseEdition(e.target.value)}
            >
              {LICENSE_EDITIONS.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </div>
          <div className="config-row">
            <label htmlFor="license-expires">到期日期</label>
            <input
              id="license-expires"
              className="input max-w-xs"
              type="date"
              value={licenseExpiresAt}
              onChange={(e) => setLicenseExpiresAt(e.target.value)}
            />
          </div>
        </div>
        <div className="flex justify-end">
          <button type="button" className="btn" disabled={saving} onClick={() => void onSaveLicense()}>
            {saving ? "保存中…" : "保存许可证"}
          </button>
        </div>
      </section>

      <section className="card stack-md">
        <div>
          <h2 className="text-lg font-semibold">工作区能力覆盖</h2>
          <p className="text-muted text-sm mt-1">
            在平台默认模块基础上，为单个工作区收紧或放开能力（例如试点空间关闭 K8s）。
          </p>
        </div>
        <div className="config-row">
          <label htmlFor="workspace-select">工作区</label>
          <select
            id="workspace-select"
            className="input max-w-md"
            value={selectedWorkspaceId}
            onChange={(e) => setSelectedWorkspaceId(e.target.value)}
          >
            {workspaces.length === 0 ? <option value="">暂无工作区</option> : null}
            {workspaces.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </div>
        {selectedWorkspaceId ? (
          <div className="stack-sm">
            {FEATURE_KEYS.map((key) => (
              <FeatureSwitch
                key={key}
                label={FEATURE_MODULE_LABELS[key]}
                checked={selectedWorkspaceOverrides[key]}
                onChange={(next) => updateWorkspaceOverride(key, next)}
              />
            ))}
          </div>
        ) : null}
        <div className="flex justify-end">
          <button
            type="button"
            className="btn"
            disabled={saving || !selectedWorkspaceId}
            onClick={() => void onSaveWorkspaceOverrides()}
          >
            {saving ? "保存中…" : "保存工作区覆盖"}
          </button>
        </div>
      </section>
    </>
  );

  if (isPlatform) {
    return (
      <div className="platform-orgs-page stack-lg">
        <PlatformPanelHead
          title="平台配置"
          description="功能模块、SSO 登录策略、许可证与工作区能力覆盖。"
        />
        {sections}
      </div>
    );
  }

  return <div className="stack-lg">{sections}</div>;
}
