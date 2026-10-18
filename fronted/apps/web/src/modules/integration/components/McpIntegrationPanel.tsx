"use client";

import { useMemo, useState } from "react";
import { useShellContext } from "@/components/layout/shell-context";

const TOOLS = [
  { name: "list_projects", desc: "列出当前工作区项目" },
  { name: "list_knowledge_bases", desc: "列出知识库" },
  { name: "list_agents", desc: "列出项目下的 Agent（需 project_id）" },
  { name: "search_knowledge", desc: "检索知识库文档" },
  { name: "chat_with_agent", desc: "与 Agent 对话并返回回复" },
] as const;

interface McpIntegrationPanelProps {
  apiBaseUrl?: string;
}

export function McpIntegrationPanel({ apiBaseUrl = "http://localhost:8080" }: McpIntegrationPanelProps) {
  const { workspaceId } = useShellContext();
  const [token, setToken] = useState("");
  const [copied, setCopied] = useState<string | null>(null);

  const backendRoot = useMemo(
    () => (apiBaseUrl.includes(":3000") ? apiBaseUrl.replace(":3000", ":8080") : apiBaseUrl),
    [apiBaseUrl],
  );

  const stdioConfig = useMemo(
    () =>
      JSON.stringify(
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
      ),
    [backendRoot, token, workspaceId],
  );

  const httpExample = useMemo(
    () => `POST ${backendRoot}/api/v1/mcp/rpc
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
}`,
    [backendRoot, token, workspaceId],
  );

  async function copy(text: string, key: string) {
    await navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  }

  return (
    <div className="mcp-integration stack-md">
      <section className="mcp-integration-block" aria-labelledby="mcp-setup-title">
        <h2 id="mcp-setup-title" className="mcp-integration-block-title">
          连接配置
        </h2>
        <p className="mcp-integration-block-desc">
          粘贴工作区 API Token 以生成本地 MCP 配置。Token 仅在浏览器内使用，不会上传服务器。
        </p>
        <label htmlFor="mcp-token" className="mcp-integration-field">
          <span className="mcp-integration-field-label">API Token</span>
          <input
            id="mcp-token"
            className="input"
            type="password"
            placeholder="粘贴工作区 API Token"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            autoComplete="off"
          />
        </label>
        <dl className="mcp-integration-meta">
          <div>
            <dt>工作区 ID</dt>
            <dd>
              <code>{workspaceId ?? "未选择工作区"}</code>
            </dd>
          </div>
          <div>
            <dt>RPC 端点</dt>
            <dd>
              <code>/api/v1/mcp/rpc</code>
            </dd>
          </div>
        </dl>
      </section>

      <section className="mcp-integration-block" aria-labelledby="mcp-tools-title">
        <h2 id="mcp-tools-title" className="mcp-integration-block-title">
          可用 Tools
        </h2>
        <p className="mcp-integration-block-desc">MCP 客户端可调用的 SparkOps 能力。</p>
        <ul className="mcp-integration-tools">
          {TOOLS.map((tool) => (
            <li key={tool.name} className="mcp-integration-tool">
              <code className="mcp-integration-tool-name">{tool.name}</code>
              <span className="mcp-integration-tool-desc">{tool.desc}</span>
            </li>
          ))}
        </ul>
      </section>

      <section className="mcp-integration-block" aria-labelledby="mcp-stdio-title">
        <div className="mcp-integration-block-head">
          <div>
            <h2 id="mcp-stdio-title" className="mcp-integration-block-title">
              Cursor / Claude Desktop
            </h2>
            <p className="mcp-integration-block-desc">stdio 模式，保存为 MCP 配置文件。</p>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => void copy(stdioConfig, "stdio")}
          >
            {copied === "stdio" ? "已复制" : "复制 mcp.json"}
          </button>
        </div>
        <pre className="mcp-integration-code mcp-integration-code--dark">{stdioConfig}</pre>
        <p className="mcp-integration-footnote">
          在仓库根目录执行，或将 command 改为编译后的二进制路径。
        </p>
      </section>

      <section className="mcp-integration-block" aria-labelledby="mcp-http-title">
        <div className="mcp-integration-block-head">
          <div>
            <h2 id="mcp-http-title" className="mcp-integration-block-title">
              HTTP JSON-RPC
            </h2>
            <p className="mcp-integration-block-desc">远程调用示例，需携带 Bearer Token 与工作区 ID。</p>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => void copy(httpExample, "http")}
          >
            {copied === "http" ? "已复制" : "复制示例"}
          </button>
        </div>
        <pre className="mcp-integration-code">{httpExample}</pre>
      </section>
    </div>
  );
}
