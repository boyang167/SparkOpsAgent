import { PageHeader } from "@/components/ui/PageHeader";

export default function WebhooksPage() {
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Config" title="Webhooks" description="CI 触发 Webhook（对应旧 /webhook/list）" />
      <section className="section stack-md max-w-2xl">
        <p className="text-sm text-muted">
          向 Go API 公网端点 POST 构建事件，请求头需携带 <code className="text-xs">X-Webhook-Secret</code>（与后端{" "}
          <code className="text-xs">WEBHOOK_SECRET</code> 环境变量一致）。
        </p>
        <pre className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4 text-xs overflow-auto">
{`POST /api/v1/public/builds/webhook
Headers:
  X-Webhook-Secret: <your-secret>
  Content-Type: application/json

Body example:
{
  "project_id": "<uuid>",
  "branch": "main",
  "event": "push"
}`}
        </pre>
        <p className="text-xs text-muted">
          事件经 AMQP 队列异步触发 Jenkins 构建（<code>delivery.driver=jenkins</code> 时生效）。
        </p>
      </section>
    </div>
  );
}
