"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  FEATURE_MODULE_LABELS,
  type FeatureModuleKey,
} from "@sparkops/contracts/platform";
import type { AuthProvidersVM } from "@sparkops/contracts/identity";
import { identity, platform, tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";

type FeatureKey = FeatureModuleKey;
type FeatureValues = Pick<
  import("@sparkops/contracts/platform").WorkspaceEntitlementsVM,
  FeatureKey
>;
type SettingsTab = "features" | "sso" | "license" | "workspace";

const FEATURE_KEYS = Object.keys(FEATURE_MODULE_LABELS) as FeatureKey[];

const FEATURE_HINTS: Partial<Record<FeatureKey, string>> = {
  k8s: "运行实例、定时任务、基础设施等依赖 Kubernetes 的能力",
  mcp: "外部集成 / Webhook / MCP 工具链",
  delivery: "构建流水线、发布与交付相关能力",
  observability: "监控概览、日志、链路追踪与指标",
};

const LICENSE_EDITIONS = [
  { value: "community", label: "Community", desc: "基础能力" },
  { value: "professional", label: "Professional", desc: "交付与可观测" },
  { value: "enterprise", label: "Enterprise", desc: "全模块 + MCP" },
] as const;

const SETTINGS_TABS: { id: SettingsTab; label: string }[] = [
  { id: "features", label: "功能模块" },
  { id: "sso", label: "单点登录" },
  { id: "license", label: "许可证" },
  { id: "workspace", label: "工作区覆盖" },
];

type WorkspaceFeatureOverrides = Record<string, Partial<Record<FeatureKey, boolean>>>;

function FeatureSwitch({
  label,
  description,
  checked,
  disabled,
  onChange,
  platformStyle = false,
}: {
  label: string;
  description?: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (next: boolean) => void;
  platformStyle?: boolean;
}) {
  const rowClass = platformStyle ? "platform-settings-switch-row" : "config-switch-row";
  return (
    <div className={rowClass}>
      <div className="platform-settings-switch-copy">
        <div className="platform-settings-switch-label">{label}</div>
        {description ? <div className="platform-settings-switch-desc">{description}</div> : null}
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

function readFeatures(config: Record<string, unknown> | undefined): FeatureValues {
  const defaults: FeatureValues = {
    agent: true,
    knowledge: true,
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
    const parsed: Partial<Record<FeatureKey, boolean>> = {};
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
  const [activeTab, setActiveTab] = useState<SettingsTab>("features");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [features, setFeatures] = useState<FeatureValues>(() => readFeatures(undefined));
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
  const enabledFeatureCount = useMemo(
    () => FEATURE_KEYS.filter((key) => features[key]).length,
    [features],
  );

  const selectedWorkspaceOverrides = useMemo(() => {
    if (!selectedWorkspaceId) {
      return {
        agent: true,
        knowledge: true,
        delivery: true,
        observability: true,
        k8s: true,
        mcp: true,
      } satisfies FeatureValues;
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

  async function onSaveFeatures() {
    await persistConfig(
      {
        ...config,
        features: { ...features },
        sso: buildSsoConfig(),
        license: buildLicenseConfig(),
        workspace_features: workspaceFeatures,
      },
      "功能模块配置已保存，菜单将按新能力过滤。",
    );
  }

  async function onSaveSso() {
    await persistConfig(
      {
        ...config,
        features: config.features ?? { ...features },
        sso: buildSsoConfig(),
        license: buildLicenseConfig(),
        workspace_features: workspaceFeatures,
      },
      "SSO 登录策略已保存。",
    );
  }

  async function onSaveLicense() {
    await persistConfig(
      {
        ...config,
        features: config.features ?? { ...features },
        sso: buildSsoConfig(),
        license: buildLicenseConfig(),
        workspace_features: workspaceFeatures,
      },
      licenseExpired ? "许可证已保存（当前已过期，部分模块将被限制）。" : "许可证已保存。",
    );
  }

  async function onSaveWorkspaceOverrides() {
    if (!selectedWorkspaceId) return;
    await persistConfig(
      {
        ...config,
        features: config.features ?? { ...features },
        sso: buildSsoConfig(),
        license: buildLicenseConfig(),
        workspace_features: workspaceFeatures,
      },
      "工作区能力覆盖已保存。",
    );
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

  function handleSaveActiveTab() {
    if (activeTab === "features") void onSaveFeatures();
    else if (activeTab === "sso") void onSaveSso();
    else if (activeTab === "license") void onSaveLicense();
    else void onSaveWorkspaceOverrides();
  }

  function saveLabelForTab(tab: SettingsTab) {
    if (tab === "features") return "保存模块配置";
    if (tab === "sso") return "保存 SSO 配置";
    if (tab === "license") return "保存许可证";
    return "保存工作区覆盖";
  }

  const featureGrid = (values: FeatureValues, onChange: (key: FeatureKey, next: boolean) => void) => (
    <div className={isPlatform ? "platform-settings-feature-grid" : "stack-sm"}>
      {FEATURE_KEYS.map((key) => (
        <FeatureSwitch
          key={key}
          platformStyle={isPlatform}
          label={FEATURE_MODULE_LABELS[key]}
          description={FEATURE_HINTS[key]}
          checked={values[key]}
          onChange={(next) => onChange(key, next)}
        />
      ))}
    </div>
  );

  const featuresSection = (
    <>
      {isPlatform ? (
        <p className="platform-settings-section-desc">
          控制业务控制台菜单与 Cmd+K 可见性。K8s 模块在未接入集群时默认关闭；接入集群后会自动启用（除非在此显式关闭）。
        </p>
      ) : (
        <div>
          <h2 className="text-lg font-semibold">功能模块</h2>
          <p className="text-muted text-sm mt-1">
            控制业务控制台菜单与 Cmd+K 可见性。K8s 模块在未接入集群时默认关闭；接入集群后会自动启用（除非在此显式关闭）。
          </p>
        </div>
      )}
      {isPlatform ? (
        <div className="platform-settings-summary">
          <span className="platform-settings-summary-label">已启用模块</span>
          <span className="platform-settings-summary-value">
            {enabledFeatureCount} / {FEATURE_KEYS.length}
          </span>
        </div>
      ) : null}
      {featureGrid(features, (key, next) => setFeatures((prev) => ({ ...prev, [key]: next })))}
      {!isPlatform ? (
        <div className="flex justify-end">
          <button type="button" className="btn" disabled={saving} onClick={() => void onSaveFeatures()}>
            {saving ? "保存中…" : "保存模块配置"}
          </button>
        </div>
      ) : null}
    </>
  );

  const ssoSection = (
    <>
      {isPlatform ? (
        <p className="platform-settings-section-desc">
          运行时 OIDC/LDAP 由服务端 config.yaml 控制；此处配置登录页展示策略。
        </p>
      ) : (
        <div>
          <h2 className="text-lg font-semibold">单点登录 (SSO)</h2>
          <p className="text-muted text-sm mt-1">
            运行时 OIDC/LDAP 由服务端 config.yaml 控制；此处配置登录页展示策略。
          </p>
        </div>
      )}
      <div className={isPlatform ? "platform-settings-status-grid" : "stack-sm text-sm"}>
        <div className="platform-settings-status-item">
          <span className="platform-settings-status-name">OIDC 运行时</span>
          <span
            className={
              providers?.oidc_enabled
                ? "platform-settings-badge platform-settings-badge-on"
                : "platform-settings-badge"
            }
          >
            {providers?.oidc_enabled ? "已启用" : "未配置"}
          </span>
        </div>
        <div className="platform-settings-status-item">
          <span className="platform-settings-status-name">LDAP 运行时</span>
          <span
            className={
              providers?.ldap_enabled
                ? "platform-settings-badge platform-settings-badge-on"
                : "platform-settings-badge"
            }
          >
            {providers?.ldap_enabled ? "已启用" : "未配置"}
          </span>
        </div>
      </div>
      <div className={isPlatform ? "platform-settings-form-stack" : "stack-sm text-sm"}>
        <FeatureSwitch
          platformStyle={isPlatform}
          label="登录页展示企业 SSO"
          description="关闭后登录页仅显示邮箱密码（若允许）"
          checked={ssoEnabled}
          onChange={setSsoEnabled}
        />
        <div className={isPlatform ? "platform-settings-field" : "config-row"}>
          <label htmlFor="sso-provider" className="platform-settings-field-label">
            默认 SSO 提供商
          </label>
          <select
            id="sso-provider"
            className="input platform-settings-field-input"
            value={ssoProvider}
            onChange={(e) => setSsoProvider(e.target.value)}
          >
            <option value="oidc">OIDC</option>
            <option value="ldap">LDAP</option>
          </select>
        </div>
        <FeatureSwitch
          platformStyle={isPlatform}
          label="允许邮箱密码登录"
          checked={showPasswordLogin}
          onChange={setShowPasswordLogin}
        />
      </div>
      {!isPlatform ? (
        <div className="flex justify-end">
          <button type="button" className="btn" disabled={saving} onClick={() => void onSaveSso()}>
            {saving ? "保存中…" : "保存 SSO 配置"}
          </button>
        </div>
      ) : null}
    </>
  );

  const licenseSection = (
    <>
      {isPlatform ? (
        <p className="platform-settings-section-desc">
          版本决定 MCP 等高级模块上限；到期后将降级为基础能力（智能体 + 知识库）。
        </p>
      ) : (
        <div>
          <h2 className="text-lg font-semibold">许可证</h2>
          <p className="text-muted text-sm mt-1">
            版本决定 MCP 等高级模块上限；到期后将降级为基础能力（智能体 + 知识库）。
          </p>
        </div>
      )}
      {licenseExpired ? (
        <InlineAlert tone="error">许可证已过期，交付/K8s/可观测等模块将被自动禁用。</InlineAlert>
      ) : null}
      {isPlatform ? (
        <div className="platform-settings-edition-grid">
          {LICENSE_EDITIONS.map((item) => (
            <button
              key={item.value}
              type="button"
              className={`platform-settings-edition-card${
                licenseEdition === item.value ? " platform-settings-edition-card-active" : ""
              }`}
              onClick={() => setLicenseEdition(item.value)}
            >
              <span className="platform-settings-edition-name">{item.label}</span>
              <span className="platform-settings-edition-desc">{item.desc}</span>
            </button>
          ))}
        </div>
      ) : (
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
      )}
      <div className={isPlatform ? "platform-settings-field" : "config-row text-sm"}>
        <label htmlFor="license-expires" className="platform-settings-field-label">
          到期日期
        </label>
        <input
          id="license-expires"
          className="input platform-settings-field-input"
          type="date"
          value={licenseExpiresAt}
          onChange={(e) => setLicenseExpiresAt(e.target.value)}
        />
      </div>
      {!isPlatform ? (
        <div className="flex justify-end">
          <button type="button" className="btn" disabled={saving} onClick={() => void onSaveLicense()}>
            {saving ? "保存中…" : "保存许可证"}
          </button>
        </div>
      ) : null}
    </>
  );

  const workspaceSection = (
    <>
      {isPlatform ? (
        <p className="platform-settings-section-desc">
          在平台默认模块基础上，为单个工作区收紧或放开能力（例如试点空间关闭 K8s）。
        </p>
      ) : (
        <div>
          <h2 className="text-lg font-semibold">工作区能力覆盖</h2>
          <p className="text-muted text-sm mt-1">
            在平台默认模块基础上，为单个工作区收紧或放开能力（例如试点空间关闭 K8s）。
          </p>
        </div>
      )}
      <div className={isPlatform ? "platform-settings-field" : "config-row"}>
        <label htmlFor="workspace-select" className="platform-settings-field-label">
          工作区
        </label>
        <select
          id="workspace-select"
          className="input platform-settings-field-input"
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
        featureGrid(selectedWorkspaceOverrides, updateWorkspaceOverride)
      ) : (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>请先选择工作区</div>
      )}
      {!isPlatform ? (
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
      ) : null}
    </>
  );

  if (loading) {
    if (isPlatform) {
      return (
        <div className="platform-orgs-page">
          <section className="platform-table-card platform-orgs-card">
            <PlatformPanelHead
              title="平台配置"
              description="功能模块、SSO 登录策略、许可证与工作区能力覆盖。"
            />
            <div className="platform-table-empty">加载平台配置…</div>
          </section>
        </div>
      );
    }
    return <div className="empty-state">加载平台配置…</div>;
  }

  if (isPlatform) {
    return (
      <div className="platform-orgs-page">
        <section className="platform-table-card platform-orgs-card">
          <PlatformPanelHead
            title="平台配置"
            description="功能模块、SSO 登录策略、许可证与工作区能力覆盖。"
          />

          {error ? (
            <div className="platform-settings-alerts">
              <InlineAlert tone="error">{error}</InlineAlert>
            </div>
          ) : null}
          {message ? (
            <div className="platform-settings-alerts">
              <InlineAlert tone="success">{message}</InlineAlert>
            </div>
          ) : null}

          <div className="platform-settings-toolbar">
            <div className="platform-orgs-tabs platform-settings-tabs" role="tablist" aria-label="配置分类">
              {SETTINGS_TABS.map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  aria-selected={activeTab === tab.id}
                  className={`platform-orgs-tab${activeTab === tab.id ? " platform-orgs-tab-active" : ""}`}
                  onClick={() => {
                    setActiveTab(tab.id);
                    setMessage(null);
                    setError(null);
                  }}
                >
                  {tab.label}
                </button>
              ))}
            </div>
          </div>

          <div className="platform-settings-body" role="tabpanel">
            {activeTab === "features" ? featuresSection : null}
            {activeTab === "sso" ? ssoSection : null}
            {activeTab === "license" ? licenseSection : null}
            {activeTab === "workspace" ? workspaceSection : null}
          </div>

          <div className="platform-settings-footer">
            <button
              type="button"
              className="btn"
              disabled={saving || (activeTab === "workspace" && !selectedWorkspaceId)}
              onClick={handleSaveActiveTab}
            >
              {saving ? "保存中…" : saveLabelForTab(activeTab)}
            </button>
          </div>
        </section>
      </div>
    );
  }

  return (
    <div className="stack-lg">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {message ? <InlineAlert tone="success">{message}</InlineAlert> : null}
      <section className="card stack-md">{featuresSection}</section>
      <section className="card stack-md">{ssoSection}</section>
      <section className="card stack-md">{licenseSection}</section>
      <section className="card stack-md">{workspaceSection}</section>
    </div>
  );
}
