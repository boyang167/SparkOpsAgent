"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useShellContext } from "@/components/layout/shell-context";

const TOOLS = [
  { name: "list_projects", desc: "列出当前工作区项目" },
  { name: "list_knowledge_bases", desc: "列出知识库" },
  { name: "list_agents", desc: "列出项目下的 Agent（需 project_id）" },
  { name: "search_knowledge", desc: "检索知识库文档" },
  { name: "chat_with_agent", desc: "与 Agent 对话并返回回复" },
];

interface McpIntegrationPanelProps {
  apiBaseUrl?: string;
}

export function McpIntegrationPanel({ apiBaseUrl = "http://localhost:8080" }: McpIntegrationPanelProps) {
  const { workspaceId } = useShellContext();
  const [token, setToken] = useState("");
  const [copied, setCopied] = useState<string | null>(null);

  const stdioConfig = useMemo(() => {
    const backendRoot = apiBaseUrl.includes(":3000")
      ? apiBaseUrl.replace(":3000", ":8080")
      : apiBaseUrl;
    return JSON.stringify(
      {
        mcpServers: {
          sparkops: {
            command: "go",
            args: ["run", "./backend/cmd/mcp-server"],
            env: {
              SPARKOPS_API_URL: backendRoot,
              SPARKOPS_TOKEN: token || "<your-api-token>",
              SPARKOPS_WORKSPACE_ID: workspaceId || "<workspace-id>",
            },
          },
        },
      },
      null,
      2,
    );
  }, [apiBaseUrl, token, workspaceId]);

  const httpExample = useMemo(() => {
    return `POST ${apiBaseUrl.replace(":3000", ":8080")}/api/v1/mcp/rpc
Authorization: Bearer ${token || "<your-api-token>"}
X-Workspace-Id: ${workspaceId || "<workspace-id>"}
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_projects",
    "arguments": {}
  }
}`;
  }, [apiBaseUrl, token, workspaceId]);

  async function copy(text: string, key: string) {
    await navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  }

  return (
    <div className="platform-orgs-page">
      <section className="platform-table-card platform-orgs-card">
        <div className="platform-orgs-head">
          <div className="platform-orgs-head-copy">
            <h1 className="platform-orgs-title">MCP 集成</h1>
            <p className="platform-orgs-desc">
              通过 Model Context Protocol 让 Cursor、Claude Desktop 等 AI 客户端调用 SparkOps 能力。
              需先在管理控制台创建 API Token，并在平台配置中启用 MCP 模块。
            </p>
          </div>
          <Link href="/admin/api-tokens" className="btn platform-add-btn">
            管理 API Token
          </Link>
        </div>

        <div className="platform-settings-body">
          <div className="platform-settings-field">
            <label htmlFor="mcp-token" className="platform-settings-field-label">
              API Token（粘贴后用于生成配置，不会上传服务器）
            </label>
            <input
              id="mcp-token"
              className="input platform-settings-field-input max-w-xl"
              type="password"
              placeholder="粘贴工作区 API Token"
              value={token}
              onChange={(e) => setToken(e.target.value)}
            />
          </div>

          <div className="platform-settings-status-grid">
            <div className="platform-settings-status-item">
              <span className="platform-settings-status-name">工作区 ID</span>
              <code className="text-xs">{workspaceId ?? "未选择工作区"}</code>
            </div>
            <div className="platform-settings-status-item">
              <span className="platform-settings-status-name">RPC 端点</span>
              <code className="text-xs">/api/v1/mcp/rpc</code>
            </div>
          </div>

          <div>
            <h2 className="text-sm font-semibold mb-2">可用 Tools</h2>
            <div className="platform-settings-feature-grid">
              {TOOLS.map((tool) => (
                <div key={tool.name} className="platform-settings-switch-row">
                  <div className="platform-settings-switch-copy">
                    <div className="platform-settings-switch-label font-mono text-sm">{tool.name}</div>
                    <div className="platform-settings-switch-desc">{tool.desc}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="stack-sm">
            <div className="flex items-center justify-between gap-2">
              <h2 className="text-sm font-semibold">Cursor / Claude Desktop（stdio）</h2>
              <button
                type="button"
                className="btn-secondary !py-1 !px-2 text-xs"
                onClick={() => void copy(stdioConfig, "stdio")}
              >
                {copied === "stdio" ? "已复制" : "复制 mcp.json"}
              </button>
            </div>
            <pre className="rounded-lg border border-[var(--border)] bg-[#0f172a] text-[#e2e8f0] p-4 text-xs overflow-auto max-h-80">
              {stdioConfig}
            </pre>
            <p className="text-xs text-muted">
              将内容保存到 Cursor 的 MCP 配置文件。本地需在仓库根目录执行，或改为编译后的二进制路径。
            </p>
          </div>

          <div className="stack-sm">
            <div className="flex items-center justify-between gap-2">
              <h2 className="text-sm font-semibold">HTTP JSON-RPC 示例</h2>
              <button
                type="button"
                className="btn-secondary !py-1 !px-2 text-xs"
                onClick={() => void copy(httpExample, "http")}
              >
                {copied === "http" ? "已复制" : "复制示例"}
              </button>
            </div>
            <pre className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4 text-xs overflow-auto max-h-64">
              {httpExample}
            </pre>
          </div>
        </div>
      </section>
    </div>
  );
}
