"use client";

import { useMemo, useState } from "react";

const WEBHOOK_PATH = "/api/v1/public/builds/webhook";

interface WebhooksConfigPanelProps {
  apiBaseUrl?: string;
}

export function WebhooksConfigPanel({ apiBaseUrl = "http://localhost:8080" }: WebhooksConfigPanelProps) {
  const [copied, setCopied] = useState(false);

  const publicBase = useMemo(
    () => (apiBaseUrl.includes(":3000") ? apiBaseUrl.replace(":3000", ":8080") : apiBaseUrl),
    [apiBaseUrl],
  );

  const requestExample = useMemo(
    () => `POST ${publicBase}${WEBHOOK_PATH}
Headers:
  X-Webhook-Secret: <your-secret>
  Content-Type: application/json

Body:
{
  "project_id": "<uuid>",
  "branch": "main",
  "event": "push"
}`,
    [publicBase],
  );

  async function copyExample() {
    await navigator.clipboard.writeText(requestExample);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="mcp-integration stack-md">
      <section className="mcp-integration-block" aria-labelledby="webhook-endpoint-title">
        <h2 id="webhook-endpoint-title" className="mcp-integration-block-title">
          触发端点
        </h2>
        <p className="mcp-integration-block-desc">
          向公网 API 发送构建事件。请求头需携带 <code>X-Webhook-Secret</code>，值与后端{" "}
          <code>WEBHOOK_SECRET</code> 环境变量一致。
        </p>
        <dl className="mcp-integration-meta">
          <div>
            <dt>方法</dt>
            <dd>
              <code>POST</code>
            </dd>
          </div>
          <div>
            <dt>路径</dt>
            <dd>
              <code>{WEBHOOK_PATH}</code>
            </dd>
          </div>
          <div>
            <dt>鉴权头</dt>
            <dd>
              <code>X-Webhook-Secret</code>
            </dd>
          </div>
        </dl>
      </section>

      <section className="mcp-integration-block" aria-labelledby="webhook-example-title">
        <div className="mcp-integration-block-head">
          <div>
            <h2 id="webhook-example-title" className="mcp-integration-block-title">
              请求示例
            </h2>
            <p className="mcp-integration-block-desc">复制后在 CI 系统或 curl 中直接使用。</p>
          </div>
          <button type="button" className="btn btn-ghost btn-sm" onClick={() => void copyExample()}>
            {copied ? "已复制" : "复制示例"}
          </button>
        </div>
        <pre className="mcp-integration-code">{requestExample}</pre>
        <p className="mcp-integration-footnote">
          事件经 AMQP 队列异步触发 Jenkins 构建（<code>delivery.driver=jenkins</code> 时生效）。
        </p>
      </section>
    </div>
  );
}
