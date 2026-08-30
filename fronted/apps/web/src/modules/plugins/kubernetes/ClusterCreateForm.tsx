"use client";

import { useState } from "react";

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
  const [showVelauxPassword, setShowVelauxPassword] = useState(false);
  const [showPrometheusPassword, setShowPrometheusPassword] = useState(false);

  return (
    <div className="k8s-create-form stack-md">
      <label>
        <span className="label">
          名称 <span className="text-danger">*</span>
        </span>
        <input
          className="input"
          value={value.name}
          onChange={(e) => onChange({ ...value, name: e.target.value })}
          required
          readOnly={nameReadOnly}
        />
      </label>
      <fieldset className="k8s-fieldset">
        <legend className="label">
          凭据类型 <span className="text-danger">*</span>
        </legend>
        <div className="flex gap-4">
          <label className="inline-flex items-center gap-2 text-sm">
            <input
              type="radio"
              checked={value.kube_type === 1}
              onChange={() => onChange({ ...value, kube_type: 1 })}
            />
            KubeConfig
          </label>
          <label className="inline-flex items-center gap-2 text-sm">
            <input
              type="radio"
              checked={value.kube_type === 2}
              onChange={() => onChange({ ...value, kube_type: 2 })}
            />
            Token
          </label>
        </div>
      </fieldset>
      <label>
        <span className="label">
          集群凭证 {credentialOptional ? null : <span className="text-danger">*</span>}
        </span>
        <textarea
          className="input font-mono text-xs"
          rows={6}
          value={value.kube_config}
          placeholder={
            credentialOptional
              ? "留空则不更新凭证"
              : value.kube_type === 1
                ? "粘贴完整 kubeconfig"
                : "粘贴 ServiceAccount Token"
          }
          onChange={(e) => onChange({ ...value, kube_config: e.target.value })}
          required={!credentialOptional}
        />
      </label>
      <label>
        <span className="label">
          接口地址 <span className="text-danger">*</span>
        </span>
        <input
          className="input"
          value={value.api_address}
          placeholder="请输入Master API 地址，例如: https://127.0.0.1:6443"
          onChange={(e) => onChange({ ...value, api_address: e.target.value })}
          required
        />
      </label>

      <div className="k8s-form-divider">AI配置</div>
      <label>
        <span className="label">接口地址</span>
        <input
          className="input"
          value={value.llm_base_url}
          placeholder="请输入接口地址, 例如: https://api.deepseek.com/v1/chat/completions"
          onChange={(e) => onChange({ ...value, llm_base_url: e.target.value })}
        />
      </label>
      <label>
        <span className="label">ApiKey</span>
        <input
          className="input"
          type="password"
          value={value.llm_api_key}
          placeholder="请输入apiKey"
          onChange={(e) => onChange({ ...value, llm_api_key: e.target.value })}
        />
      </label>
      <label>
        <span className="label">模型名称</span>
        <input
          className="input"
          value={value.llm_model}
          placeholder="请输入模型名称, 例如: deepseek-chat"
          onChange={(e) => onChange({ ...value, llm_model: e.target.value })}
        />
      </label>

      <div className="k8s-form-divider">VelaUX多集群管理</div>
      <label>
        <span className="label">VelaUX地址</span>
        <input
          className="input"
          value={value.velaux_url}
          placeholder="请输入VelaUX接口地址, 例如: http://127.0.0.1:8000"
          onChange={(e) => onChange({ ...value, velaux_url: e.target.value })}
        />
      </label>
      <label>
        <span className="label">VelaUX用户</span>
        <input
          className="input"
          value={value.velaux_user}
          placeholder="请输入VelaUX认证用户"
          onChange={(e) => onChange({ ...value, velaux_user: e.target.value })}
        />
      </label>
      <label>
        <span className="label">VelaUX密码</span>
        <div className="relative">
          <input
            className="input pr-10"
            type={showVelauxPassword ? "text" : "password"}
            value={value.velaux_password}
            placeholder="请输入VelaUX认证密码"
            onChange={(e) => onChange({ ...value, velaux_password: e.target.value })}
          />
          <button
            type="button"
            className="k8s-eye-btn"
            onClick={() => setShowVelauxPassword((v) => !v)}
            aria-label="切换密码可见"
          >
            {showVelauxPassword ? "隐" : "显"}
          </button>
        </div>
      </label>

      <div className="k8s-form-divider">Prometheus监控</div>
      <label>
        <span className="label">监控地址</span>
        <input
          className="input"
          value={value.prometheus_url}
          placeholder="请输入普罗米修斯监控地址, 例如: http://127.0.0.1:9090"
          onChange={(e) => onChange({ ...value, prometheus_url: e.target.value })}
        />
      </label>
      <fieldset className="k8s-fieldset">
        <legend className="label">
          认证类型 <span className="text-danger">*</span>
        </legend>
        <div className="flex gap-4">
          <label className="inline-flex items-center gap-2 text-sm">
            <input
              type="radio"
              checked={value.prometheus_auth_type === 1}
              onChange={() => onChange({ ...value, prometheus_auth_type: 1, prometheus_user: "", prometheus_password: "" })}
            />
            无
          </label>
          <label className="inline-flex items-center gap-2 text-sm">
            <input
              type="radio"
              checked={value.prometheus_auth_type === 2}
              onChange={() => onChange({ ...value, prometheus_auth_type: 2 })}
            />
            BaseAuth
          </label>
        </div>
      </fieldset>
      {value.prometheus_auth_type === 2 ? (
        <>
          <label>
            <span className="label">用户名</span>
            <input
              className="input"
              value={value.prometheus_user}
              placeholder="请输入普罗米修斯认证用户"
              onChange={(e) => onChange({ ...value, prometheus_user: e.target.value })}
            />
          </label>
          <label>
            <span className="label">密码</span>
            <div className="relative">
              <input
                className="input pr-10"
                type={showPrometheusPassword ? "text" : "password"}
                value={value.prometheus_password}
                placeholder="请输入普罗米修斯认证密码"
                onChange={(e) => onChange({ ...value, prometheus_password: e.target.value })}
              />
              <button
                type="button"
                className="k8s-eye-btn"
                onClick={() => setShowPrometheusPassword((v) => !v)}
                aria-label="切换密码可见"
              >
                {showPrometheusPassword ? "隐" : "显"}
              </button>
            </div>
          </label>
        </>
      ) : null}
    </div>
  );
}
