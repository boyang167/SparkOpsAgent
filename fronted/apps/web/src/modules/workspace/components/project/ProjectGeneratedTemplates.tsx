"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { GeneratedTemplateArtifact, ProjectVM } from "@sparkops/contracts/workspace";
import { generateProjectTemplates } from "./project-template-render";

interface ProjectGeneratedTemplatesProps {
  workspaceId: string;
  project: ProjectVM;
  /** review：审核弹窗只读预览，隐藏刷新/复制/下载 */
  variant?: "default" | "review";
}

export function ProjectGeneratedTemplates({
  workspaceId,
  project,
  variant = "default",
}: ProjectGeneratedTemplatesProps) {
  const [artifacts, setArtifacts] = useState<GeneratedTemplateArtifact[]>([]);
  const [activeKey, setActiveKey] = useState("");
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setMessage(null);
    try {
      const items = await workspace.listGeneratedTemplates(workspaceId, project.id);
      setArtifacts(items);
      setActiveKey((prev) => {
        if (items.some((item) => item.key === prev)) return prev;
        return items[0]?.key ?? "";
      });
      if (items.length === 0) {
        setMessage("请先在第 3 步选择至少一个模板");
      }
    } catch (err) {
      // Fallback to local generation if API unavailable
      try {
        const next = generateProjectTemplates(project, []);
        setArtifacts(next);
        setActiveKey(next[0]?.key ?? "");
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
  }, [workspaceId, project]);

  useEffect(() => {
    void load();
  }, [load]);

  const active = useMemo(
    () => artifacts.find((item) => item.key === activeKey) ?? artifacts[0],
    [artifacts, activeKey],
  );

  async function handleCopy() {
    if (!active?.content) return;
    try {
      await navigator.clipboard.writeText(active.content);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setMessage("复制失败，请手动选择文本复制");
    }
  }

  function handleDownload() {
    if (!active?.content) return;
    const blob = new Blob([active.content], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = active.filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  const isReview = variant === "review";

  if (loading && artifacts.length === 0) {
    return <p className="text-muted">{isReview ? "加载模板中…" : "生成模板中…"}</p>;
  }

  if (artifacts.length === 0) {
    return (
      <div className="project-generated-empty">
        <p className="text-muted">
          {message || (isReview ? "暂无生成模板，请确认项目已完成模板配置。" : "暂无已选模板，完成第 3 步配置后可在此生成配置文件。")}
        </p>
        {!isReview ? (
          <button type="button" className="btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
            刷新模板
          </button>
        ) : null}
      </div>
    );
  }

  return (
    <div className={`project-generated${isReview ? " project-generated-review" : ""}`}>
      {!isReview ? (
        <div className="project-generated-toolbar">
          <button
            type="button"
            className="btn project-btn-primary text-sm"
            onClick={() => void load()}
            disabled={loading}
          >
            {loading ? "刷新中…" : "刷新模板"}
          </button>
        </div>
      ) : null}

      <div className="project-generated-tabs">
        {artifacts.map((item) => (
          <button
            key={item.key}
            type="button"
            className={item.key === active?.key ? "is-active" : ""}
            onClick={() => setActiveKey(item.key)}
          >
            {item.title}
          </button>
        ))}
      </div>

      {active ? (
        <div className="project-generated-panel">
          <div className="project-generated-panel-head">
            <div className="project-generated-panel-title">
              <strong>{active.title}</strong>
              <span className="project-tag project-tag-blue">{active.title}</span>
              {active.template_name ? (
                <span className="text-muted text-sm">来源：{active.template_name}</span>
              ) : null}
            </div>
            <div className="project-generated-actions">
              {!isReview ? (
                <>
                  <button type="button" className="btn project-btn-primary text-sm" onClick={() => void handleCopy()}>
                    {copied ? "已复制" : "复制"}
                  </button>
                  <button type="button" className="btn-secondary text-sm" onClick={handleDownload}>
                    下载
                  </button>
                </>
              ) : null}
            </div>
          </div>
          <pre className={`project-generated-code${isReview ? " project-generated-code-review" : ""}`}>
            <code>{active.content}</code>
          </pre>
        </div>
      ) : null}

      {message ? <p className="project-form-hint">{message}</p> : null}
    </div>
  );
}
