"use client";

import { useCallback, useEffect, useState } from "react";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";

interface ProjectDockerfileViewerProps {
  workspaceId: string;
  project: ProjectVM;
}

export function ProjectDockerfileViewer({ workspaceId, project }: ProjectDockerfileViewerProps) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const dockerfilePath = project.dockerfile_path?.trim() || "./Dockerfile";
  const gitUrl = project.git_url?.trim() ?? "";
  const secretId = project.secret_id?.trim() ?? "";
  const gitRef = project.git_ref_value?.trim() ?? "";
  const canLoad = Boolean(gitUrl && secretId && gitRef);

  const load = useCallback(async () => {
    if (!canLoad) return;
    setLoading(true);
    setError(null);
    try {
      const data = await workspace.getGitFileContent(workspaceId, {
        git_url: gitUrl,
        secret_id: secretId,
        ref: gitRef,
        path: dockerfilePath,
      });
      setContent(data.content);
    } catch (err) {
      setContent("");
      setError(
        err instanceof BffError
          ? err.message
          : err instanceof Error
            ? err.message
            : "加载 Dockerfile 失败",
      );
    } finally {
      setLoading(false);
    }
  }, [workspaceId, gitUrl, secretId, gitRef, dockerfilePath, canLoad]);

  useEffect(() => {
    void load();
  }, [load]);

  async function handleCopy() {
    if (!content) return;
    try {
      await navigator.clipboard.writeText(content);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setError("复制失败，请手动选择文本复制");
    }
  }

  if (!gitUrl) {
    return <p className="text-muted">请先配置 Git 仓库地址。</p>;
  }
  if (!secretId) {
    return <p className="text-muted">请先配置 Git 认证密钥。</p>;
  }
  if (!gitRef) {
    return <p className="text-muted">请先配置 Git 分支、标签或提交。</p>;
  }

  if (loading && !content) {
    return <p className="text-muted">正在加载 Dockerfile…</p>;
  }

  if (error && !content) {
    return (
      <div className="project-generated-empty">
        <p className="project-form-hint">{error}</p>
        <button type="button" className="btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
          重新加载
        </button>
      </div>
    );
  }

  return (
    <div className="project-generated">
      <div className="project-generated-panel">
        <div className="project-generated-panel-head">
          <div className="project-generated-panel-title">
            <strong>{dockerfilePath}</strong>
            <span className="project-tag project-tag-blue">Dockerfile</span>
            <span className="text-muted text-sm">
              {project.git_ref_type || "branch"}: {gitRef}
            </span>
          </div>
          <div className="project-generated-actions">
            <button type="button" className="btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
              {loading ? "刷新中…" : "刷新"}
            </button>
            <button
              type="button"
              className="btn project-btn-primary text-sm"
              onClick={() => void handleCopy()}
              disabled={!content}
            >
              {copied ? "已复制" : "复制"}
            </button>
          </div>
        </div>
        <pre className="project-generated-code">
          <code>{content || "（文件为空）"}</code>
        </pre>
      </div>
      {error ? <p className="project-form-hint">{error}</p> : null}
    </div>
  );
}
