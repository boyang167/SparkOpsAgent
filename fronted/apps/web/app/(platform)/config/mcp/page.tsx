import { McpIntegrationPanel } from "@/modules/integration/components/McpIntegrationPanel";

export default function McpIntegrationPage() {
  const apiBaseUrl =
    process.env.NEXT_PUBLIC_API_URL ??
    process.env.UPSTREAM_API_URL ??
    "http://localhost:8080";

  return (
    <div className="platform-page">
      <McpIntegrationPanel apiBaseUrl={apiBaseUrl} />
    </div>
  );
}
