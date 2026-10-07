"use client";

import { useCallback, useEffect, useState } from "react";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";

const INSTRUCTION_RE =
  /^(\s*)(FROM|RUN|CMD|LABEL|EXPOSE|ENV|ADD|COPY|ENTRYPOINT|VOLUME|USER|WORKDIR|ARG|ONBUILD|STOPSIGNAL|HEALTHCHECK|SHELL|MAINTAINER)(\s.*|$)/i;

interface ProjectDockerfileViewerProps {
  workspaceId: string;
  project: ProjectVM;
  /** 默认折叠，展开时才请求文件内容 */
  lazy?: boolean;
}

export function ProjectDockerfileViewer({ workspaceId, project, lazy = false }: ProjectDockerfileViewerProps) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [expanded, setExpanded] = useState(!lazy);

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
      setLoaded(true);
    }
  }, [workspaceId, gitUrl, secretId, gitRef, dockerfilePath, canLoad]);

  useEffect(() => {
    setLoaded(false);
    setContent("");
    setError(null);
  }, [load]);

  useEffect(() => {
    if (expanded && !loaded && !loading) void load();
  }, [expanded, loaded, loading, load]);

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

  const missing = !gitUrl
    ? "配置 Git 仓库地址后即可预览 Dockerfile。"
    : !secretId
      ? "配置 Git 认证密钥后即可预览 Dockerfile。"
      : !gitRef
        ? "配置 Git 分支、标签或提交后即可预览 Dockerfile。"
        : null;

  if (missing) {
    return <div className="df-viewer-state">{missing}</div>;
  }

  const lines = content ? content.replace(/\n$/, "").split("\n") : [];

  const title = (
    <>
      {lazy ? (
        <svg className="df-viewer-chevron" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M9 6l6 6-6 6" />
        </svg>
      ) : null}
      <span className="df-viewer-path">{dockerfilePath}</span>
      <span className="df-viewer-ref">
        {project.git_ref_type || "branch"} · {gitRef}
      </span>
      {expanded && lines.length > 0 ? <span className="df-viewer-meta">{lines.length} 行</span> : null}
      {lazy && !expanded ? <span className="df-viewer-meta">点击查看</span> : null}
    </>
  );

  return (
    <div className={`df-viewer${lazy ? " is-lazy" : ""}${expanded ? " is-expanded" : ""}`}>
      <div className="df-viewer-head">
        {lazy ? (
          <button
            type="button"
            className="df-viewer-title df-viewer-toggle"
            onClick={() => setExpanded((v) => !v)}
            aria-expanded={expanded}
          >
            {title}
          </button>
        ) : (
          <div className="df-viewer-title">{title}</div>
        )}
        {expanded ? (
          <div className="df-viewer-actions">
            <button
              type="button"
              className="df-viewer-btn"
              onClick={() => void load()}
              disabled={loading}
              aria-label="刷新"
              title="刷新"
            >
              <svg className={loading ? "is-spinning" : undefined} viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7" />
              </svg>
            </button>
            <button type="button" className="df-viewer-btn is-text" onClick={() => void handleCopy()} disabled={!content}>
              {copied ? "已复制" : "复制"}
            </button>
          </div>
        ) : null}
      </div>

      {!expanded ? null : loading && !content ? (
        <div className="df-viewer-loading" role="status" aria-label="加载中">
          {Array.from({ length: 6 }).map((_, i) => (
            <span key={i} style={{ width: `${40 + ((i * 37) % 50)}%` }} />
          ))}
        </div>
      ) : error && !content ? (
        <div className="df-viewer-loading is-empty is-error">
          <span>{error}</span>
          <button type="button" className="btn-secondary" onClick={() => void load()} disabled={loading}>
            重新加载
          </button>
        </div>
      ) : lines.length === 0 ? (
        <div className="df-viewer-loading is-empty">文件为空</div>
      ) : (
        <pre className="df-viewer-code">
          <code>
            {lines.map((line, i) => {
              if (line.trimStart().startsWith("#")) {
                return <span key={i} className="df-line is-comment">{line}</span>;
              }
              const match = INSTRUCTION_RE.exec(line);
              return (
                <span key={i} className="df-line">
                  {match ? (
                    <>
                      {match[1]}
                      <span className="df-kw">{match[2]}</span>
                      {match[3]}
                    </>
                  ) : (
                    line || " "
                  )}
                </span>
              );
            })}
          </code>
        </pre>
      )}
      {expanded && error && content ? <p className="df-viewer-error">{error}</p> : null}
    </div>
  );
}
