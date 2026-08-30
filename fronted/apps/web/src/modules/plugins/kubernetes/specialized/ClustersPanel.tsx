import type { KubernetesCluster } from "../ClusterPicker";

export function ClustersPanel({ clusters }: { clusters: KubernetesCluster[] }) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>ID</th>
            <th>Server</th>
            <th>Prometheus</th>
            <th>Velaux</th>
            <th>LLM</th>
          </tr>
        </thead>
        <tbody>
          {clusters.map((item) => (
            <tr key={item.id}>
              <td>{item.name || "-"}</td>
              <td>{item.id}</td>
              <td>{item.server || "-"}</td>
              <td>{item.prometheus_url || "-"}</td>
              <td>{item.velaux_url || "-"}</td>
              <td>{item.llm_model || "-"}</td>
            </tr>
          ))}
          {clusters.length === 0 ? (
            <tr>
              <td colSpan={6}>
                <div className="empty-state">暂无集群</div>
              </td>
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  );
}
