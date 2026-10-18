"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { integration } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { McpToolTier, McpToolVM } from "@sparkops/contracts/mcp";
import { useShellContext } from "@/components/layout/shell-context";
import { InlineAlert } from "@/components/ui/InlineAlert";

const TIER_LABELS: Record<McpToolTier, string> = {
  discovery: "发现",
  read: "检索",
  action: "执行",
};

const TIER_ORDER: McpToolTier[] = ["discovery", "read", "action"];

interface McpIntegrationPanelProps {
  apiBaseUrl?: string;
}

function toolTier(tool: McpToolVM): McpToolTier {
  return tool.tier ?? "read";
}

export function McpIntegrationPanel({ apiBaseUrl = "http://localhost:8080" }: McpIntegrationPanelProps) {
  const { workspaceId } = useShellContext();
  const [token, setToken] = useState("");
  const [copied, setCopied] = useState<string | null>(null);
  const [tools, setTools] = useState<McpToolVM[]>([]);
  const [toolsLoading, setToolsLoading] = useState(true);
  const [toolsError, setToolsError] = useState<string | null>(null);

  const backendRoot = useMemo(
    () => (apiBaseUrl.includes(":3000") ? apiBaseUrl.replace(":3000", ":8080") : apiBaseUrl),
    [apiBaseUrl],
  );

  const loadTools = useCallback(async () => {
    if (!workspaceId) {
      setTools([]);
      setToolsLoading(false);
      return;
    }
    setToolsLoading(true);
    setToolsError(null);
    try {
      setTools(await integration.listMcpTools(workspaceId));
    } catch (err) {
      setTools([]);
      setToolsError(err instanceof BffError ? err.message : "加载 Tools 失败");
    } finally {
      setToolsLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void loadTools();
  }, [loadTools]);

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

{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}`,
    [backendRoot, token, workspaceId],
  );

  const groupedTools = useMemo(() => {
    const groups = new Map<McpToolTier, McpToolVM[]>();
    for (const tier of TIER_ORDER) {
      groups.set(tier, []);
    }
    for (const tool of tools) {
      const tier = toolTier(tool);
      groups.get(tier)?.push(tool);
    }
    return TIER_ORDER.map((tier) => ({ tier, items: groups.get(tier) ?? [] })).filter(
      (group) => group.items.length > 0,
    );
  }, [tools]);

  async function copy(text: string, key: string) {
    await navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  }

  return (
    <div className="mcp-integration">
      <section className="mcp-integration-block" aria-labelledby="mcp-setup-title">
        <h2 id="mcp-setup-title" className="mcp-integration-block-title">
          生成 Cursor / Claude 配置
        </h2>
        <p className="mcp-integration-block-desc">
          粘贴 API Token 后复制配置到 MCP 客户端。Token 仅在本地使用，不会上传。
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
            <dt>工作区</dt>
            <dd>
              <code>{workspaceId ?? "未选择"}</code>
            </dd>
          </div>
          <div>
            <dt>端点</dt>
            <dd>
              <code>/api/v1/mcp/rpc</code>
            </dd>
          </div>
        </dl>

        <div className="mcp-integration-actions">
          <button type="button" className="btn btn-sm" onClick={() => void copy(stdioConfig, "stdio")}>
            {copied === "stdio" ? "已复制" : "复制 mcp.json"}
          </button>
        </div>

        <details className="mcp-integration-details">
          <summary>预览配置</summary>
          <pre className="mcp-integration-code mcp-integration-code--dark">{stdioConfig}</pre>
        </details>
      </section>

      <details className="mcp-integration-details mcp-integration-details--flat">
        <summary>
          可用 Tools
          {toolsLoading ? "…" : `（${tools.length}）`}
        </summary>
        {toolsError ? <InlineAlert tone="error">{toolsError}</InlineAlert> : null}
        {toolsLoading ? (
          <p className="mcp-integration-block-desc">加载中…</p>
        ) : tools.length === 0 ? (
          <p className="mcp-integration-block-desc">当前权限或模块配置下暂无可用 Tools。</p>
        ) : (
          <div className="mcp-integration-tool-groups">
            {groupedTools.map((group) => (
              <div key={group.tier} className="mcp-integration-tool-group">
                <p className="mcp-integration-tool-group-label">{TIER_LABELS[group.tier]}</p>
                <ul className="mcp-integration-tool-list">
                  {group.items.map((tool) => (
                    <li key={tool.name}>
                      <code>{tool.name}</code>
                      <span>{tool.description}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        )}
      </details>

      <details className="mcp-integration-details mcp-integration-details--flat">
        <summary>HTTP JSON-RPC</summary>
        <div className="mcp-integration-details-body">
          <p className="mcp-integration-block-desc">远程调用时需携带 Bearer Token 与 X-Workspace-Id。</p>
          <div className="mcp-integration-actions">
            <button type="button" className="btn-ghost btn-sm" onClick={() => void copy(httpExample, "http")}>
              {copied === "http" ? "已复制" : "复制示例"}
            </button>
          </div>
          <pre className="mcp-integration-code">{httpExample}</pre>
        </div>
      </details>
    </div>
  );
}
