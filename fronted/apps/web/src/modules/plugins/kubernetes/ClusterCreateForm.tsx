"use client";

import { useState, type ReactNode } from "react";

export type ClusterCreateFormValue = {
  name: string;
  kube_type: 1 | 2;
  kube_config: string;
  api_address: string;
  llm_base_url: string;
  llm_api_key: string;
  llm_model: string;
  velaux_url: string;
  velaux_user: string;
  velaux_password: string;
  prometheus_url: string;
  prometheus_auth_type: 1 | 2;
  prometheus_user: string;
  prometheus_password: string;
};

export const EMPTY_CLUSTER_CREATE_FORM: ClusterCreateFormValue = {
  name: "",
  kube_type: 1,
  kube_config: "",
  api_address: "",
  llm_base_url: "",
  llm_api_key: "",
  llm_model: "",
  velaux_url: "",
  velaux_user: "",
  velaux_password: "",
  prometheus_url: "",
  prometheus_auth_type: 1,
  prometheus_user: "",
  prometheus_password: "",
};

export function formFromCluster(cluster: {
  name: string;
  server?: string;
  prometheus_url?: string;
  prometheus_user?: string;
  prometheus_password?: string;
  velaux_url?: string;
  velaux_user?: string;
  velaux_password?: string;
  llm_base_url?: string;
  llm_api_key?: string;
  llm_model?: string;
}): ClusterCreateFormValue {
  return {
    ...EMPTY_CLUSTER_CREATE_FORM,
    name: cluster.name ?? "",
    api_address: cluster.server ?? "",
    llm_base_url: cluster.llm_base_url ?? "",
    llm_api_key: cluster.llm_api_key ?? "",
    llm_model: cluster.llm_model ?? "",
    velaux_url: cluster.velaux_url ?? "",
    velaux_user: cluster.velaux_user ?? "",
    velaux_password: cluster.velaux_password ?? "",
    prometheus_url: cluster.prometheus_url ?? "",
    prometheus_auth_type: cluster.prometheus_user ? 2 : 1,
    prometheus_user: cluster.prometheus_user ?? "",
    prometheus_password: cluster.prometheus_password ?? "",
  };
}

export function clusterSettingsPayload(form: ClusterCreateFormValue) {
  return {
    prometheus_url: form.prometheus_url,
    prometheus_user: form.prometheus_auth_type === 2 ? form.prometheus_user : "",
    prometheus_password: form.prometheus_auth_type === 2 ? form.prometheus_password : "",
    velaux_url: form.velaux_url,
    velaux_user: form.velaux_user,
    velaux_password: form.velaux_password,
    llm_base_url: form.llm_base_url,
    llm_api_key: form.llm_api_key,
    llm_model: form.llm_model,
  };
}

function hasLlmConfig(value: ClusterCreateFormValue) {
  return Boolean(value.llm_base_url.trim() || value.llm_api_key.trim() || value.llm_model.trim());
}

function hasVelauxConfig(value: ClusterCreateFormValue) {
  return Boolean(value.velaux_url.trim() || value.velaux_user.trim() || value.velaux_password.trim());
}

function hasPrometheusConfig(value: ClusterCreateFormValue) {
  return Boolean(
    value.prometheus_url.trim() ||
      value.prometheus_user.trim() ||
      value.prometheus_password.trim() ||
      value.prometheus_auth_type === 2,
  );
}

function FormField({
  label,
  required,
  hint,
  children,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="k8s-form-field">
      <span className="label">
        {label}
        {required ? <span className="text-danger"> *</span> : null}
      </span>
      {children}
      {hint ? <span className="k8s-form-field-hint">{hint}</span> : null}
    </label>
  );
}

function SegmentedControl<T extends string | number>({
  value,
  options,
  onChange,
  ariaLabel,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
  ariaLabel: string;
}) {
  return (
    <div className="k8s-segmented" role="radiogroup" aria-label={ariaLabel}>
      {options.map((option) => (
        <button
          key={String(option.value)}
          type="button"
          role="radio"
          aria-checked={value === option.value}
          className={`k8s-segmented-btn${value === option.value ? " is-active" : ""}`}
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

function PasswordField({
  value,
  placeholder,
  onChange,
}: {
  value: string;
  placeholder: string;
  onChange: (next: string) => void;
}) {
  const [visible, setVisible] = useState(false);

  return (
    <div className="k8s-password-field">
      <input
        className="input"
        type={visible ? "text" : "password"}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
      />
      <button
        type="button"
        className="k8s-eye-btn"
        onClick={() => setVisible((v) => !v)}
        aria-label={visible ? "隐藏密码" : "显示密码"}
      >
        {visible ? (
          <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
            <path
              d="M2.5 10s2.5-5 7.5-5 7.5 5 7.5 5-2.5 5-7.5 5-7.5-5-7.5-5Z"
              stroke="currentColor"
              strokeWidth="1.5"
            />
            <circle cx="10" cy="10" r="2.25" stroke="currentColor" strokeWidth="1.5" />
          </svg>
        ) : (
          <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
            <path
              d="M3 3l14 14M7.2 7.7A3.5 3.5 0 0 0 10 13.5c1.1 0 2.08-.52 2.72-1.33M5.6 5.9C4.2 6.9 3 8.4 2.5 10c0 0 2.5 5 7.5 5 1.4 0 2.68-.38 3.76-1.04"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
        )}
      </button>
    </div>
  );
}

function CollapsibleSection({
  title,
  hint,
  defaultOpen = false,
  children,
}: {
  title: string;
  hint?: string;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <section className="k8s-cluster-form-collapsible">
      <button
        type="button"
        className="k8s-cluster-form-collapsible-head"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <div className="k8s-cluster-form-collapsible-copy">
          <span className="k8s-cluster-form-section-title">{title}</span>
          {hint ? <span className="k8s-cluster-form-section-hint">{hint}</span> : null}
        </div>
        <svg
          className={`k8s-cluster-form-collapsible-chevron${open ? " is-open" : ""}`}
          viewBox="0 0 20 20"
          fill="none"
          aria-hidden="true"
        >
          <path d="M5 8l5 5 5-5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>
      {open ? <div className="k8s-cluster-form-collapsible-body stack-sm">{children}</div> : null}
    </section>
  );
}

export function ClusterCreateForm({
  value,
  onChange,
  nameReadOnly = false,
  credentialOptional = false,
}: {
  value: ClusterCreateFormValue;
  onChange: (next: ClusterCreateFormValue) => void;
  nameReadOnly?: boolean;
  credentialOptional?: boolean;
}) {
  return (
    <div className="k8s-cluster-form">
      <section className="k8s-cluster-form-section">
        <header className="k8s-cluster-form-section-head">
          <h3 className="k8s-cluster-form-section-title">连接配置</h3>
        </header>
        <div className="k8s-cluster-form-section-body stack-sm">
          <FormField label="名称" required>
            <input
              className={`input${nameReadOnly ? " is-readonly" : ""}`}
              value={value.name}
              onChange={(e) => onChange({ ...value, name: e.target.value })}
              required
              readOnly={nameReadOnly}
            />
          </FormField>

          <FormField label="凭据类型" required>
            <SegmentedControl
              ariaLabel="凭据类型"
              value={value.kube_type}
              options={[
                { value: 1 as const, label: "KubeConfig" },
                { value: 2 as const, label: "Token" },
              ]}
              onChange={(kube_type) => onChange({ ...value, kube_type })}
            />
          </FormField>

          <FormField label="集群凭证" required={!credentialOptional}>
            <textarea
              className="input font-mono text-xs k8s-cluster-credential-input"
              rows={5}
              value={value.kube_config}
              placeholder={
                credentialOptional
                  ? "留空保留原值"
                  : value.kube_type === 1
                    ? "kubeconfig"
                    : "Token"
              }
              onChange={(e) => onChange({ ...value, kube_config: e.target.value })}
              required={!credentialOptional}
            />
          </FormField>

          <FormField label="接口地址" required>
            <input
              className="input"
              value={value.api_address}
              placeholder="https://127.0.0.1:6443"
              onChange={(e) => onChange({ ...value, api_address: e.target.value })}
              required
            />
          </FormField>
        </div>
      </section>

      <CollapsibleSection title="AI 配置" hint="可选" defaultOpen={hasLlmConfig(value)}>
        <FormField label="接口地址">
          <input
            className="input"
            value={value.llm_base_url}
            placeholder="https://api.deepseek.com/v1/chat/completions"
            onChange={(e) => onChange({ ...value, llm_base_url: e.target.value })}
          />
        </FormField>
        <FormField label="ApiKey">
          <PasswordField
            value={value.llm_api_key}
            placeholder="请输入 ApiKey"
            onChange={(llm_api_key) => onChange({ ...value, llm_api_key })}
          />
        </FormField>
        <FormField label="模型名称">
          <input
            className="input"
            value={value.llm_model}
            placeholder="deepseek-chat"
            onChange={(e) => onChange({ ...value, llm_model: e.target.value })}
          />
        </FormField>
      </CollapsibleSection>

      <CollapsibleSection title="VelaUX 多集群管理" hint="可选" defaultOpen={hasVelauxConfig(value)}>
        <FormField label="VelaUX 地址">
          <input
            className="input"
            value={value.velaux_url}
            placeholder="http://127.0.0.1:8000"
            onChange={(e) => onChange({ ...value, velaux_url: e.target.value })}
          />
        </FormField>
        <FormField label="VelaUX 用户">
          <input
            className="input"
            value={value.velaux_user}
            placeholder="认证用户"
            onChange={(e) => onChange({ ...value, velaux_user: e.target.value })}
          />
        </FormField>
        <FormField label="VelaUX 密码">
          <PasswordField
            value={value.velaux_password}
            placeholder="认证密码"
            onChange={(velaux_password) => onChange({ ...value, velaux_password })}
          />
        </FormField>
      </CollapsibleSection>

      <CollapsibleSection title="Prometheus 监控" hint="可选" defaultOpen={hasPrometheusConfig(value)}>
        <FormField label="监控地址">
          <input
            className="input"
            value={value.prometheus_url}
            placeholder="http://127.0.0.1:9090"
            onChange={(e) => onChange({ ...value, prometheus_url: e.target.value })}
          />
        </FormField>
        <FormField label="认证类型" required>
          <SegmentedControl
            ariaLabel="Prometheus 认证类型"
            value={value.prometheus_auth_type}
            options={[
              { value: 1 as const, label: "无" },
              { value: 2 as const, label: "BaseAuth" },
            ]}
            onChange={(prometheus_auth_type) =>
              onChange({
                ...value,
                prometheus_auth_type,
                prometheus_user: prometheus_auth_type === 1 ? "" : value.prometheus_user,
                prometheus_password: prometheus_auth_type === 1 ? "" : value.prometheus_password,
              })
            }
          />
        </FormField>
        {value.prometheus_auth_type === 2 ? (
          <>
            <FormField label="用户名">
              <input
                className="input"
                value={value.prometheus_user}
                placeholder="认证用户"
                onChange={(e) => onChange({ ...value, prometheus_user: e.target.value })}
              />
            </FormField>
            <FormField label="密码">
              <PasswordField
                value={value.prometheus_password}
                placeholder="认证密码"
                onChange={(prometheus_password) => onChange({ ...value, prometheus_password })}
              />
            </FormField>
          </>
        ) : null}
      </CollapsibleSection>
    </div>
  );
}
