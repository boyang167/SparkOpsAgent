"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { agent, integration } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { AgentVersionVM } from "@sparkops/contracts/agent";
import type { KnowledgeBaseVM } from "@sparkops/contracts/knowledge";
import type { ModelVM } from "@sparkops/contracts/integration";

interface AgentVersionPanelProps {
  agentId: string;
  workspaceId: string;
  versions: AgentVersionVM[];
  isPublished: boolean;
  knowledgeBases: KnowledgeBaseVM[];
}

export function AgentVersionPanel({
  agentId,
  workspaceId,
  versions,
  isPublished,
  knowledgeBases,
}: AgentVersionPanelProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [systemPrompt, setSystemPrompt] = useState(
    versions[versions.length - 1]?.system_prompt ?? "You are a helpful assistant.",
  );
  const [modelName, setModelName] = useState(versions[versions.length - 1]?.model_name ?? "");
  const [selectedKbIds, setSelectedKbIds] = useState<string[]>(
    versions[versions.length - 1]?.knowledge_base_ids ?? [],
  );
  const [models, setModels] = useState<ModelVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    integration
      .listModels()
      .then((items) => {
        setModels(items);
        if (items.length > 0 && !modelName) {
          setModelName(items[0].id);
        }
      })
      .catch(() => {
        setModels([{ id: "gpt-4o-mini", provider: "openai" }]);
      });
  }, [open, modelName]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await agent.addAgentVersion(agentId, workspaceId, {
        system_prompt: systemPrompt,
        model_name: modelName,
        knowledge_base_ids: selectedKbIds,
      });
      setOpen(false);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="section stack-md">
      <div className="section-header !mb-0">
        <div>
          <h2 className="section-title">版本配置</h2>
          <p className="text-muted mt-1">
            {isPublished ? "已发布 Agent 可追加新版本后重新发布" : "编辑 Prompt 与模型，发布前可多次迭代"}
          </p>
        </div>
        {!open ? (
          <button className="btn-secondary" type="button" onClick={() => setOpen(true)}>
            新建版本
          </button>
        ) : null}
      </div>

      {versions.length === 0 ? (
        <div className="empty-state">暂无版本记录</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>版本</th>
                <th>模型</th>
                <th>知识库</th>
                <th>System Prompt</th>
              </tr>
            </thead>
            <tbody>
              {versions.map((ver) => (
                <tr key={ver.id}>
                  <td className="font-medium">v{ver.version}</td>
                  <td>
                    <span className="badge badge-primary">{ver.model_name}</span>
                  </td>
                  <td>
                    {ver.knowledge_base_ids && ver.knowledge_base_ids.length > 0 ? (
                      <span className="text-sm text-muted">{ver.knowledge_base_ids.length} 个</span>
                    ) : (
                      <span className="text-sm text-muted">—</span>
                    )}
                  </td>
                  <td>
                    <p className="text-sm line-clamp-3 whitespace-pre-wrap">{ver.system_prompt}</p>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {open ? (
        <form className="form-panel space-y-4" onSubmit={onSubmit}>
          <div className="flex items-center justify-between gap-4">
            <h3 className="section-title">新建版本 v{versions.length + 1}</h3>
            <button className="btn-ghost !py-1 !px-2" type="button" onClick={() => setOpen(false)}>
              取消
            </button>
          </div>
          <div>
            <label className="label" htmlFor="version-model">
              模型
            </label>
            {models.length > 0 ? (
              <select
                id="version-model"
                className="select"
                value={modelName}
                onChange={(e) => setModelName(e.target.value)}
                required
              >
                {models.map((item) => (
                  <option key={`${item.provider}-${item.id}`} value={item.id}>
                    {item.id} ({item.provider})
                  </option>
                ))}
              </select>
            ) : (
              <input
                id="version-model"
                className="input"
                value={modelName}
                onChange={(e) => setModelName(e.target.value)}
                required
              />
            )}
          </div>
          {knowledgeBases.length > 0 ? (
            <div>
              <label className="label">绑定知识库（RAG）</label>
              <div className="stack-sm mt-2">
                {knowledgeBases.map((kb) => (
                  <label key={kb.id} className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={selectedKbIds.includes(kb.id)}
                      onChange={(e) => {
                        if (e.target.checked) {
                          setSelectedKbIds((prev) => [...prev, kb.id]);
                        } else {
                          setSelectedKbIds((prev) => prev.filter((id) => id !== kb.id));
                        }
                      }}
                    />
                    {kb.name}
                  </label>
                ))}
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted">当前 Workspace 暂无知识库，可在「知识库」菜单创建后绑定。</p>
          )}
          <div>
            <label className="label" htmlFor="version-prompt">
              System Prompt
            </label>
            <textarea
              id="version-prompt"
              className="input min-h-32 resize-y font-mono text-sm"
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
              required
            />
          </div>
          {error ? <p className="error">{error}</p> : null}
          <button className="btn" type="submit" disabled={loading || !modelName}>
            {loading ? "保存中..." : "保存版本"}
          </button>
        </form>
      ) : null}
    </section>
  );
}
