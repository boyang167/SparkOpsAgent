export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="auth-shell">
      <aside className="auth-brand">
        <div>
          <div className="sidebar-brand-mark">
            <span className="sidebar-logo">S</span>
            SparkOps
          </div>
          <p className="text-muted mt-6 max-w-sm">
            清爽、简约的 Agent 运营平台。从 Workspace 到 Project、Agent 对话，一条链路打通。
          </p>
        </div>
        <div className="text-muted text-sm">
          <p>Org → Workspace → Project → Agent</p>
          <p className="mt-2 opacity-70">BFF 架构 · 安全 Cookie · RBAC 治理</p>
        </div>
      </aside>
      <div className="auth-panel">
        <div className="w-full max-w-md">{children}</div>
      </div>
    </div>
  );
}
