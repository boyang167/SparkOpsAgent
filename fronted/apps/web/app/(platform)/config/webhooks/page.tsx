import { PageHeader } from "@/components/ui/PageHeader";
import { WebhooksConfigPanel } from "@/modules/integration/components/WebhooksConfigPanel";

export default function WebhooksPage() {
  const apiBaseUrl =
    process.env.NEXT_PUBLIC_API_URL ??
    process.env.UPSTREAM_API_URL ??
    "http://localhost:8080";

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Config"
        title="外部集成"
        description="通过 Webhook 从 CI 系统触发 SparkOps 构建流水线。"
      />
      <WebhooksConfigPanel apiBaseUrl={apiBaseUrl} />
    </div>
  );
}
