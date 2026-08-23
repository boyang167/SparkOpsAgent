export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="auth-shell">
      <aside className="auth-brand" aria-label="SparkOps 品牌介绍">
        <div className="auth-brand-glow auth-brand-glow-a" aria-hidden />
        <div className="auth-brand-glow auth-brand-glow-b" aria-hidden />
        <div className="auth-brand-inner">
          <div className="auth-brand-head">
            <div className="auth-brand-logo">
              <span className="auth-brand-logo-mark">S</span>
              <span className="auth-brand-logo-text">SparkOps</span>
            </div>
            <p className="auth-brand-tagline">
              企业级 Agent 运营平台
            </p>
            <p className="auth-brand-desc">
              从组织治理到智能体交付，为私有化部署而生的统一控制台。
            </p>
          </div>

          <ul className="auth-brand-features">
            <li className="auth-brand-feature">
              <span className="auth-brand-feature-icon" aria-hidden>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                  <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
                </svg>
              </span>
              <div>
                <p className="auth-brand-feature-title">多租户 IAM</p>
                <p className="auth-brand-feature-desc">Org · Workspace · RBAC 全链路治理</p>
              </div>
            </li>
            <li className="auth-brand-feature">
              <span className="auth-brand-feature-icon" aria-hidden>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                  <path d="M4 19V5M4 19h16M8 15v-4M12 15V9M16 15v-2" strokeLinecap="round" />
                </svg>
              </span>
              <div>
                <p className="auth-brand-feature-title">Agent 全生命周期</p>
                <p className="auth-brand-feature-desc">构建、发布、运行与可观测一体化</p>
              </div>
            </li>
            <li className="auth-brand-feature">
              <span className="auth-brand-feature-icon" aria-hidden>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                  <rect x="3" y="11" width="18" height="10" rx="2" />
                  <path d="M7 11V7a5 5 0 0110 0v4" strokeLinecap="round" />
                </svg>
              </span>
              <div>
                <p className="auth-brand-feature-title">企业安全接入</p>
                <p className="auth-brand-feature-desc">BFF · Cookie 会话 · OIDC / LDAP</p>
              </div>
            </li>
          </ul>
        </div>

        <p className="auth-brand-foot">© SparkOps · Private Agent Platform</p>
      </aside>

      <div className="auth-panel">
        <div className="auth-panel-mobile-brand">
          <span className="auth-brand-logo-mark">S</span>
          <span>SparkOps</span>
        </div>
        <div className="auth-panel-content">{children}</div>
      </div>
    </div>
  );
}
