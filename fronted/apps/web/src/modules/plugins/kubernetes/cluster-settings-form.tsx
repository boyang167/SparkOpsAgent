import type { KubernetesCluster } from "./cluster-types";

export const EMPTY_CLUSTER_SETTINGS = {
  prometheus_url: "",
  prometheus_user: "",
  prometheus_password: "",
  velaux_url: "",
  velaux_user: "",
  velaux_password: "",
  llm_base_url: "",
  llm_api_key: "",
  llm_model: "",
};

export type ClusterSettingsFormValue = typeof EMPTY_CLUSTER_SETTINGS;

export function settingsFromCluster(cluster: KubernetesCluster | undefined): ClusterSettingsFormValue {
  if (!cluster) return EMPTY_CLUSTER_SETTINGS;
  return {
    prometheus_url: cluster.prometheus_url ?? "",
    prometheus_user: cluster.prometheus_user ?? "",
    prometheus_password: cluster.prometheus_password ?? "",
    velaux_url: cluster.velaux_url ?? "",
    velaux_user: cluster.velaux_user ?? "",
    velaux_password: cluster.velaux_password ?? "",
    llm_base_url: cluster.llm_base_url ?? "",
    llm_api_key: cluster.llm_api_key ?? "",
    llm_model: cluster.llm_model ?? "",
  };
}

export function ClusterSettingsForm({
  settings,
  onChange,
}: {
  settings: ClusterSettingsFormValue;
  onChange: (next: ClusterSettingsFormValue) => void;
}) {
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {field("Prometheus URL", "prometheus_url", settings, onChange)}
      {field("Prometheus 用户", "prometheus_user", settings, onChange)}
      {field("Prometheus 密码", "prometheus_password", settings, onChange, true)}
      {field("Velaux URL", "velaux_url", settings, onChange)}
      {field("Velaux 用户", "velaux_user", settings, onChange)}
      {field("Velaux 密码", "velaux_password", settings, onChange, true)}
      {field("LLM Base URL", "llm_base_url", settings, onChange)}
      {field("LLM API Key", "llm_api_key", settings, onChange, true)}
      {field("LLM Model", "llm_model", settings, onChange)}
    </div>
  );
}

function field(
  label: string,
  key: keyof ClusterSettingsFormValue,
  settings: ClusterSettingsFormValue,
  onChange: (next: ClusterSettingsFormValue) => void,
  secret = false,
) {
  return (
    <label key={key}>
      <span className="label">{label}</span>
      <input
        className="input"
        type={secret ? "password" : "text"}
        value={settings[key]}
        placeholder={secret ? "***" : undefined}
        onChange={(e) => onChange({ ...settings, [key]: e.target.value })}
      />
    </label>
  );
}
