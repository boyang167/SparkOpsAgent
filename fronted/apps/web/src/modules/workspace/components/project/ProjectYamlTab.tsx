"use client";

import { useCallback, useEffect, useState } from "react";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { GeneratedTemplateArtifact, ProjectVM } from "@sparkops/contracts/workspace";
import { Modal } from "@/components/ui/Modal";
import { generateProjectTemplates } from "./project-template-render";

interface ProjectYamlTabProps {
  workspaceId: string;
  project: ProjectVM;
  active: boolean;
}

function yamlArtifacts(items: GeneratedTemplateArtifact[]) {
  return items.filter((item) => item.kind !== "jenkinsFile");
}

export function ProjectYamlTab({ workspaceId, project, active }: ProjectYamlTabProps) {
  const [artifacts, setArtifacts] = useState<GeneratedTemplateArtifact[]>([]);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [editing, setEditing] = useState<GeneratedTemplateArtifact | null>(null);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setMessage(null);
    try {
      const items = yamlArtifacts(await workspace.listGeneratedTemplates(workspaceId, project.id));
      setArtifacts(items);
      if (items.length === 0) {
        setMessage("请先在配置中选择 Service 或 Deployment 模板");
      }
    } catch (err) {
      try {
        const next = yamlArtifacts(generateProjectTemplates(project, []));
        setArtifacts(next);
        setMessage(
          err instanceof BffError
            ? `${err.message}（已使用本地兜底生成）`
            : "后端生成失败，已使用本地兜底生成",
        );
      } catch {
        setArtifacts([]);
        setMessage(err instanceof BffError ? err.message : "生成模板失败");
      }
    } finally {
      setLoading(false);
    }
  }, [project, workspaceId]);

  useEffect(() => {
    if (!active) return;
    void load();
  }, [active, load]);

  function openEditor(item: GeneratedTemplateArtifact) {
    setEditing(item);
    setDraft(item.content);
    setSaveError(null);
  }

  function closeEditor() {
    if (saving) return;
    setEditing(null);
    setDraft("");
    setSaveError(null);
  }

  async function handleSave() {
    if (!editing) return;
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await workspace.updateGeneratedTemplate(workspaceId, project.id, editing.kind, {
        content: draft,
      });
      setArtifacts((current) =>
        current.map((item) => (item.key === editing.key ? updated : item)),
      );
      setEditing(null);
      setDraft("");
      setMessage(null);
    } catch (err) {
      setSaveError(err instanceof BffError ? err.message : "保存 Yaml 失败");
    } finally {
      setSaving(false);
    }
  }

  if (loading && artifacts.length === 0) {
    return <p className="text-muted">Yaml 模板加载中…</p>;
  }

  if (artifacts.length === 0) {
    return (
      <div className="project-yaml-empty">
        <p className="text-muted">{message || "暂无已选模板，完成模板配置后可在此查看 Yaml。"}</p>
        <button type="button" className="btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
          刷新
        </button>
      </div>
    );
  }

  return (
    <>
      <div className="project-yaml-list">
        {artifacts.map((item) => (
          <article key={item.key} className="project-yaml-card">
            <header className="project-yaml-card-head">
              <h4>{item.title}</h4>
              <button
                type="button"
                className="project-yaml-edit"
                aria-label={`编辑 ${item.title}`}
                onClick={() => openEditor(item)}
              >
                ✎
              </button>
            </header>
            <pre className="project-yaml-code">
              <code>{item.content}</code>
            </pre>
          </article>
        ))}
        {message ? <p className="project-form-hint">{message}</p> : null}
      </div>

      <Modal
        open={Boolean(editing)}
        onClose={closeEditor}
        title={editing ? `编辑 ${editing.title}` : "编辑 Yaml"}
        description="修改后的 Yaml 将用于后续发布部署。"
        size="xl"
        panelClassName="project-yaml-edit-modal"
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={closeEditor} disabled={saving}>
              取消
            </button>
            <button
              type="button"
              className="btn project-btn-primary text-sm"
              onClick={() => void handleSave()}
              disabled={saving || !draft.trim()}
            >
              {saving ? "保存中…" : "保存"}
            </button>
          </>
        }
      >
        <div className="stack-sm">
          <textarea
            className="project-yaml-editor"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            spellCheck={false}
            rows={24}
          />
          {saveError ? <p className="error">{saveError}</p> : null}
        </div>
      </Modal>
    </>
  );
}
