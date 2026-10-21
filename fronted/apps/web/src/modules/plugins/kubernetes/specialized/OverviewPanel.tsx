export function OverviewPanel() {
  return (
    <div className="grid-cards">
      {[
        ["资源目录", "覆盖核心、apps、batch、networking、Gateway API、RBAC 与常见 CRD。"],
        ["集群选择", "集群来源于当前工作区授权列表，选择持久化在本地。"],
        ["诊断入口", "Chat、CloudTTY、Arthas、Velaux 均通过同源 BFF 路径访问。"],
      ].map(([name, desc]) => (
        <div key={name} className="list-item">
          <div>
            <div className="font-medium">{name}</div>
            <div className="text-sm text-muted mt-1">{desc}</div>
          </div>
        </div>
      ))}
    </div>
  );
}
