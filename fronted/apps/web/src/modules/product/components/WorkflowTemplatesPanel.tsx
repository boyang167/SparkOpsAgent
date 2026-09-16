"use client";

import { useCallback, useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { WorkflowTemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface WorkflowTemplatesPanelProps {
  workspaceId: string;
}

const DEFAULT_DEFINITION = {
  stages: [
    {
      name: "构建",
      parallel: true,
      jobs: [{ name: "build-services", type: "sparkops-build", spec: { branch: "main" } }],
    },
  ],
};

export function WorkflowTemplatesPanel({ workspaceId }: WorkflowTemplatesPanelProps) {
  const [items, setItems] = useState<WorkflowTemplateVM[]>([]);
  const [selected, setSelected] = useState<WorkflowTemplateVM | null>(null);
  const [editorTab, setEditorTab] = useState<"yaml" | "info">("info");
  const [yamlText, setYamlText] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createKey, setCreateKey] = useState("");
  const [createDesc, setCreateDesc] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await product.listWorkflowTemplates(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载模板失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function selectTemplate(tpl: WorkflowTemplateVM) {
    setSelected(tpl);
    setEditorTab("info");
    setError(null);
    try {
      const detail = await product.getWorkflowTemplate(workspaceId, tpl.id);
      const def = detail?.definition ?? tpl.definition ?? DEFAULT_DEFINITION;
      setYamlText(JSON.stringify(def, null, 2));
    } catch {
      setYamlText(JSON.stringify(tpl.definition ?? DEFAULT_DEFINITION, null, 2));
    }
  }

  async function onCreate() {
    if (!createName.trim() || !createKey.trim()) return;
    setBusy(true);
    setError(null);
    try {
      let definition: Record<string, unknown>;
      try {
        definition = JSON.parse(yamlText || JSON.stringify(DEFAULT_DEFINITION)) as Record<string, unknown>;
      } catch {
        setError("Definition 必须是合法 JSON");
        return;
      }
      await product.createWorkflowTemplate(workspaceId, {
        name: createName.trim(),
        key: createKey.trim(),
        description: createDesc.trim() || undefined,
        definition,
      });
      setShowCreate(false);
      setCreateName("");
      setCreateKey("");
      setCreateDesc("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建模板失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSave() {
    if (!selected || selected.scope !== "workspace") return;
    setBusy(true);
    setError(null);
    try {
      let definition: Record<string, unknown>;
      try {
        definition = JSON.parse(yamlText) as Record<string, unknown>;
      } catch {
        setError("Definition 必须是合法 JSON");
        return;
      }
      await product.updateWorkflowTemplate(workspaceId, selected.id, { definition });
      await load();
      const updated = items.find((i) => i.id === selected.id);
      if (updated) setSelected(updated);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function onPublish() {
    if (!selected || selected.scope !== "workspace") return;
    setBusy(true);
    setError(null);
    try {
      await product.publishWorkflowTemplate(workspaceId, selected.id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "发布失败");
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <div className="empty-state">加载中…</div>;

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          className="btn-primary text-sm"
          onClick={() => {
            setShowCreate((v) => {
              if (!v) setYamlText(JSON.stringify(DEFAULT_DEFINITION, null, 2));
              return !v;
            });
          }}
        >
          {showCreate ? "取消" : "新建 Workspace 模板"}
        </button>
      </div>

      {showCreate ? (
        <div className="rounded border p-4 stack-sm text-sm">
          <div className="grid gap-3 md:grid-cols-2">
            <label className="stack-xs">
              <span className="text-muted-foreground">名称</span>
              <input className="input" value={createName} onChange={(e) => setCreateName(e.target.value)} />
            </label>
            <label className="stack-xs">
              <span className="text-muted-foreground">标识 key</span>
              <input className="input" value={createKey} onChange={(e) => setCreateKey(e.target.value)} />
            </label>
          </div>
          <label className="stack-xs">
            <span className="text-muted-foreground">描述</span>
            <input className="input" value={createDesc} onChange={(e) => setCreateDesc(e.target.value)} />
          </label>
          <label className="stack-xs">
            <span className="text-muted-foreground">Definition JSON</span>
            <textarea
              className="input font-mono text-xs min-h-48"
              value={yamlText || JSON.stringify(DEFAULT_DEFINITION, null, 2)}
              onChange={(e) => setYamlText(e.target.value)}
            />
          </label>
          <button type="button" className="btn-primary text-sm w-fit" disabled={busy} onClick={() => void onCreate()}>
            创建草稿
          </button>
        </div>
      ) : null}

      {items.length === 0 ? (
        <div className="empty-state">暂无工作流模板</div>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="table-wrap">
            <table className="data-table text-sm">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>范围</th>
                  <th>状态</th>
                </tr>
              </thead>
              <tbody>
                {items.map((row) => (
                  <tr
                    key={row.id}
                    className={selected?.id === row.id ? "bg-muted/40 cursor-pointer" : "cursor-pointer"}
                    onClick={() => void selectTemplate(row)}
                  >
                    <td className="font-medium">{row.name}</td>
                    <td>{row.scope}</td>
                    <td>{row.status ?? "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="stack-sm border rounded p-3">
            {selected ? (
              <>
                <div className="flex justify-between items-start gap-2">
                  <div>
                    <h3 className="font-medium">{selected.name}</h3>
                    <p className="text-xs text-muted-foreground">
                      {selected.key} · v{selected.version ?? "-"} · {selected.scope}
                    </p>
                  </div>
                  {selected.scope === "workspace" ? (
                    <div className="flex gap-1">
                      <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onSave()}>
                        保存
                      </button>
                      {selected.status === "draft" ? (
                        <button type="button" className="btn-primary text-xs" disabled={busy} onClick={() => void onPublish()}>
                          发布
                        </button>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                <div className="flex gap-2 text-xs border-b pb-2">
                  <button
                    type="button"
                    className={editorTab === "info" ? "font-medium text-primary" : "text-muted-foreground"}
                    onClick={() => setEditorTab("info")}
                  >
                    概览
                  </button>
                  <button
                    type="button"
                    className={editorTab === "yaml" ? "font-medium text-primary" : "text-muted-foreground"}
                    onClick={() => setEditorTab("yaml")}
                  >
                    YAML/JSON
                  </button>
                </div>
                {editorTab === "info" ? (
                  <p className="text-sm text-muted-foreground">{selected.description || "无描述"}</p>
                ) : selected.scope === "workspace" ? (
                  <textarea
                    className="input font-mono text-xs min-h-72 w-full"
                    value={yamlText}
                    onChange={(e) => setYamlText(e.target.value)}
                  />
                ) : (
                  <pre className="max-h-96 overflow-auto rounded border bg-muted/30 p-3 text-xs">{yamlText || "—"}</pre>
                )}
              </>
            ) : (
              <div className="empty-state text-sm">选择模板查看或编辑</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
