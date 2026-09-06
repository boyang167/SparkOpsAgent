"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { secret, workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { GeneratedTemplateArtifact, ProjectVM } from "@sparkops/contracts/workspace";
import { Modal } from "@/components/ui/Modal";

interface ProjectJenkinsPreviewProps {
  workspaceId: string;
  project: ProjectVM;
  active: boolean;
}

function findJenkinsArtifact(items: GeneratedTemplateArtifact[]) {
  return items.find((item) => item.kind === "jenkinsFile") ?? null;
}

export function ProjectJenkinsPreview({ workspaceId, project, active }: ProjectJenkinsPreviewProps) {
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [artifact, setArtifact] = useState<GeneratedTemplateArtifact | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const jenkinsSecret = useMemo(
    () => secrets.find((item) => item.id === project.jenkins_secret_id),
    [project.jenkins_secret_id, secrets],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [secretList, artifacts] = await Promise.all([
        secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
        workspace.listGeneratedTemplates(workspaceId, project.id),
      ]);
      setSecrets(secretList);
      const jenkins = findJenkinsArtifact(artifacts);
      setArtifact(jenkins);
      if (!jenkins?.content) {
        setError("暂无 Jenkinsfile 模板，请先在配置中选择 Jenkinsfile 模板。");
      }
    } catch (err) {
      setArtifact(null);
      setError(err instanceof BffError ? err.message : "加载 Jenkins 配置失败");
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    if (!active) return;
    void load();
  }, [active, load]);

  function openEditor() {
    if (!artifact) return;
    setDraft(artifact.content);
    setSaveError(null);
    setEditing(true);
  }

  function closeEditor() {
    if (saving) return;
    setEditing(false);
    setDraft("");
    setSaveError(null);
  }

  async function handleSave() {
    if (!artifact) return;
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await workspace.updateGeneratedTemplate(workspaceId, project.id, artifact.kind, {
        content: draft,
      });
      setArtifact(updated);
      setError(null);
      setEditing(false);
      setDraft("");
    } catch (err) {
      setSaveError(err instanceof BffError ? err.message : "保存 Pipeline 失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="stack-md">
      <section className="project-unpublished-card">
        <h4 className="project-unpublished-card-title">Jenkins 配置</h4>
        <dl className="project-unpublished-grid">
          <div>
            <dt>项目名称</dt>
            <dd>
              {project.name}.{project.namespace || workspaceId}
            </dd>
          </div>
          <div>
            <dt>Git 地址</dt>
            <dd>{project.git_url || "-"}</dd>
          </div>
          <div>
            <dt>Git 类型</dt>
            <dd>{project.git_ref_type || "branch"}</dd>
          </div>
          <div>
            <dt>dockerfile_path</dt>
            <dd>{project.dockerfile_path || "./Dockerfile"}</dd>
          </div>
          <div>
            <dt>Jenkins 密钥</dt>
            <dd>
              {jenkinsSecret
                ? jenkinsSecret.display_name || jenkinsSecret.name
                : project.jenkins_secret_id || "-"}
            </dd>
          </div>
          <div>
            <dt>静态扫描</dt>
            <dd>{project.resources?.enable_static_scan ? "已启用" : "未启用"}</dd>
          </div>
        </dl>
      </section>

      <section className="project-yaml-card">
        <header className="project-yaml-card-head">
          <h4>Pipeline</h4>
          {artifact ? (
            <button
              type="button"
              className="project-yaml-edit"
              aria-label="编辑 Pipeline"
              onClick={openEditor}
            >
              ✎
            </button>
          ) : null}
        </header>
        {loading ? <p className="text-muted">加载中…</p> : null}
        {error && !artifact?.content ? <p className="project-form-hint">{error}</p> : null}
        {artifact?.content ? (
          <pre className="project-yaml-code">
            <code>{artifact.content}</code>
          </pre>
        ) : null}
      </section>

      <Modal
        open={editing}
        onClose={closeEditor}
        title="编辑 Pipeline"
        description="修改后的 Jenkinsfile 将用于后续构建发布。"
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
    </div>
  );
}
