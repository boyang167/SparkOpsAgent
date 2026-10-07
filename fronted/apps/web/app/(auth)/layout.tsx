export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="auth-shell">
      <aside className="auth-brand" aria-label="SparkOps 品牌介绍">
        <div className="auth-brand-inner">
          <div className="auth-brand-logo">
            <span className="auth-brand-logo-mark">S</span>
            <span className="auth-brand-logo-text">SparkOps</span>
          </div>

          <div className="auth-brand-content">
            <div className="auth-brand-story">
              <p className="auth-brand-kicker">Private Agent Platform</p>
              <p className="auth-brand-title">
                构建、交付、治理
                <br />
                企业级 Agent
              </p>
              <p className="auth-brand-desc">
                在一个统一控制台中连接团队、运行环境与交付流程。
              </p>
            </div>

            <section className="auth-brand-workflow" aria-labelledby="auth-workflow-title">
              <div className="auth-brand-workflow-head">
                <p id="auth-workflow-title">Agent delivery</p>
                <span>统一生命周期</span>
              </div>
              <ol className="auth-brand-flow" aria-label="Agent 生命周期">
                {[
                  { label: "构建", description: "模型、工具与工作流" },
                  { label: "发布", description: "版本与环境编排" },
                  { label: "运行", description: "弹性执行与调度" },
                  { label: "观测", description: "质量、成本与审计" },
                ].map((item, index) => (
                  <li className="auth-brand-flow-step" key={item.label}>
                    <span className="auth-brand-flow-index">
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <span className="auth-brand-flow-copy">
                      <strong>{item.label}</strong>
                      <small>{item.description}</small>
                    </span>
                  </li>
                ))}
              </ol>
            </section>
          </div>

          <div className="auth-brand-trust" aria-label="平台能力">
            <span>私有化部署</span>
            <span>企业身份接入</span>
            <span>审计治理</span>
          </div>
        </div>
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
