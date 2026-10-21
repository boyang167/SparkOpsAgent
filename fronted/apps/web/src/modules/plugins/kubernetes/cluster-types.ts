export type KubernetesCluster = {
  id: string;
  name: string;
  server: string;
  prometheus_url?: string;
  prometheus_user?: string;
  prometheus_password?: string;
  velaux_url?: string;
  velaux_user?: string;
  velaux_password?: string;
  llm_base_url?: string;
  llm_api_key?: string;
  llm_model?: string;
};
