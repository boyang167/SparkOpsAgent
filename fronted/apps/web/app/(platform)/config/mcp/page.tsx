import Link from "next/link";
import { PageHeader } from "@/components/ui/PageHeader";
import { McpIntegrationPanel } from "@/modules/integration/components/McpIntegrationPanel";

export default function McpIntegrationPage() {
  const apiBaseUrl =
    process.env.NEXT_PUBLIC_API_URL ??
    process.env.UPSTREAM_API_URL ??
    "http://localhost:8080";

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Config"
        title="MCP 集成"
        description="通过 Model Context Protocol 让 Cursor、Claude Desktop 等 AI 客户端调用 SparkOps 能力。"
        actions={
          <Link href="/admin/api-tokens" className="btn btn-ghost btn-sm">
            管理 API Token
          </Link>
        }
      />
      <McpIntegrationPanel apiBaseUrl={apiBaseUrl} />
    </div>
  );
}
