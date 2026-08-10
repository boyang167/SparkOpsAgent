"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { agent, integration } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ModelVM } from "@sparkops/contracts/integration";

interface CreateAgentFormProps {
  workspaceId: string;
  projectId: string;
}

export function CreateAgentForm({ workspaceId, projectId }: CreateAgentFormProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("You are a helpful assistant.");
  const [modelName, setModelName] = useState("");
  const [models, setModels] = useState<ModelVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

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
        if (!modelName) {
          setModelName("gpt-4o-mini");
        }
      });
  }, [open, modelName]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const created = await agent.createAgent(workspaceId, projectId, {
        name,
        system_prompt: systemPrompt,
        model_name: modelName,
      });
      setOpen(false);
      setName("");
      router.push(`/projects/${projectId}/agents/${created.id}`);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setLoading(false);
    }
  }

  if (!open) {
    return (
      <button className="btn" type="button" onClick={() => setOpen(true)}>
        新建 Agent
      </button>
    );
  }

  return (
    <form className="form-panel space-y-4 mb-6" onSubmit={onSubmit}>
      <div className="flex items-center justify-between gap-4">
        <h3 className="section-title">新建 Agent</h3>
        <button className="btn-ghost !py-1 !px-2" type="button" onClick={() => setOpen(false)}>
          取消
        </button>
      </div>
      <div>
        <label className="label" htmlFor="agent-name">
          名称
        </label>
        <input
          id="agent-name"
          className="input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>
      <div>
        <label className="label" htmlFor="agent-model">
          模型
        </label>
        {models.length > 0 ? (
          <select
            id="agent-model"
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
            id="agent-model"
            className="input"
            value={modelName}
            onChange={(e) => setModelName(e.target.value)}
            placeholder="加载模型列表..."
            required
          />
        )}
      </div>
      <div>
        <label className="label" htmlFor="agent-prompt">
          System Prompt
        </label>
        <textarea
          id="agent-prompt"
          className="input min-h-28 resize-y font-mono text-sm"
          value={systemPrompt}
          onChange={(e) => setSystemPrompt(e.target.value)}
          required
        />
      </div>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn" type="submit" disabled={loading || !modelName}>
        {loading ? "创建中..." : "创建"}
      </button>
    </form>
  );
}
