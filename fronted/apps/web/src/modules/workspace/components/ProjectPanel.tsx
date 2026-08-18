"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ProjectCard } from "./project/ProjectCard";
import { ProjectCreateModal } from "./project/ProjectCreateModal";
import {
  formatDateTime,
  labelOf,
  languageColor,
  languageLabel,
  PROJECT_LANGUAGE_OPTIONS,
  PROJECT_STATUS_OPTIONS,
  projectDisplayName,
  statusBadgeClass,
} from "./project/project-utils";

interface ProjectPanelProps {
  workspaceId: string;
  initialCreateOpen?: boolean;
  /** When create was opened via ?create=1, close clears this path instead of always /projects */
  createQueryPath?: string;
}

type ViewMode = "card" | "table";

const PAGE_SIZES = [6, 9, 12, 24] as const;

export function ProjectPanel({
  workspaceId,
  initialCreateOpen = false,
  createQueryPath = "/projects",
}: ProjectPanelProps) {
  const router = useRouter();
  const [items, setItems] = useState<ProjectVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(initialCreateOpen);
  const [viewMode, setViewMode] = useState<ViewMode>("card");
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [languageFilter, setLanguageFilter] = useState("");
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<(typeof PAGE_SIZES)[number]>(9);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await workspace.listProjects(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载项目失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase();
    return items.filter((item) => {
      if (statusFilter && item.status !== statusFilter) return false;
      if (languageFilter && item.language !== languageFilter) return false;
      if (namespaceFilter && !(item.namespace ?? "").includes(namespaceFilter.trim())) return false;
      if (!kw) return true;
      return [item.name, item.display_name, item.description, item.namespace]
        .some((v) => v?.toLowerCase().includes(kw));
    });
  }, [items, keyword, statusFilter, languageFilter, namespaceFilter]);

  useEffect(() => {
    setPage(1);
  }, [keyword, statusFilter, languageFilter, namespaceFilter, pageSize, viewMode]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = useMemo(() => {
    const current = Math.min(page, totalPages);
    const start = (current - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, page, pageSize, totalPages]);

  async function onArchive(item: ProjectVM) {
    if (!window.confirm(`确定归档项目「${projectDisplayName(item)}」？`)) return;
    setError(null);
    try {
      await workspace.archiveProject(workspaceId, item.id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "归档失败");
    }
  }

  function handleCreateSuccess() {
    void load();
  }

  return (
    <div className="project-panel">
      <div className="project-page-header">
        <div>
          <h2 className="project-page-title">项目管理</h2>
          <p className="project-page-desc">管理产品下的项目</p>
        </div>
        <button type="button" className="btn project-btn-primary" onClick={() => setCreateOpen(true)}>
          + 创建项目
        </button>
      </div>

      <div className="project-search-bar">
        <label className="project-inline-field">
          <span>关键词</span>
          <input
            className="input"
            placeholder="请输入项目名称或描述"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
        </label>
        <label className="project-inline-field">
          <span>状态</span>
          <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="">请选择状态</option>
            {PROJECT_STATUS_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <label className="project-inline-field">
          <span>开发语言</span>
          <select className="input" value={languageFilter} onChange={(e) => setLanguageFilter(e.target.value)}>
            <option value="">请选择开发语言</option>
            {PROJECT_LANGUAGE_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <label className="project-inline-field">
          <span>命名空间</span>
          <input
            className="input"
            placeholder="请输入命名空间"
            value={namespaceFilter}
            onChange={(e) => setNamespaceFilter(e.target.value)}
          />
        </label>
        <button
          type="button"
          className="btn-secondary"
          onClick={() => {
            setKeyword("");
            setStatusFilter("");
            setLanguageFilter("");
            setNamespaceFilter("");
          }}
        >
          重置
        </button>
      </div>

      <div className="project-view-controls">
        <div className="project-view-switch">
          <button
            type="button"
            className={viewMode === "card" ? "is-active" : ""}
            onClick={() => setViewMode("card")}
          >
            卡片视图
          </button>
          <button
            type="button"
            className={viewMode === "table" ? "is-active" : ""}
            onClick={() => setViewMode("table")}
          >
            表格视图
          </button>
        </div>
        <button type="button" className="btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
          ↻ 刷新
        </button>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="project-loading">加载中…</div>
      ) : filtered.length === 0 ? (
        <div className="project-empty">
          <p>暂无项目</p>
          <button type="button" className="btn project-btn-primary" onClick={() => setCreateOpen(true)}>
            创建第一个项目
          </button>
        </div>
      ) : (
        <>
          {viewMode === "card" ? (
            <div className="project-card-grid">
              {paged.map((project) => (
                <ProjectCard key={project.id} project={project} onArchive={(p) => void onArchive(p)} />
              ))}
            </div>
          ) : (
            <div className="project-table-shell">
              <table className="data-table project-table">
                <thead>
                  <tr>
                    <th>项目名称</th>
                    <th>命名空间</th>
                    <th>语言</th>
                    <th>状态</th>
                    <th>Git 地址</th>
                    <th>创建时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paged.map((project) => (
                    <tr key={project.id}>
                      <td>
                        <div className="project-table-name">
                          <span
                            className="project-lang-icon is-sm"
                            style={{ backgroundColor: languageColor(project.language) }}
                          >
                            {languageLabel(project.language).charAt(0).toUpperCase()}
                          </span>
                          <div>
                            <div className="font-medium">{projectDisplayName(project)}</div>
                            <div className="text-xs text-muted-foreground">{project.name}</div>
                          </div>
                        </div>
                      </td>
                      <td>{project.namespace || "-"}</td>
                      <td>{languageLabel(project.language)}</td>
                      <td>
                        <span className={statusBadgeClass(project.status)}>
                          {labelOf(PROJECT_STATUS_OPTIONS, project.status)}
                        </span>
                      </td>
                      <td className="project-url-cell" title={project.git_url}>
                        {project.git_url || "-"}
                      </td>
                      <td>{formatDateTime(project.created_at)}</td>
                      <td>
                        <div className="project-row-actions">
                          <Link href={`/projects/${project.id}`}>进入</Link>
                          <Link href={`/projects/${project.id}?configure=1`}>继续配置</Link>
                          <button type="button" className="is-danger" onClick={() => void onArchive(project)}>
                            归档
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <div className="project-pagination">
            <span className="project-pagination-total">共 {filtered.length} 个项目</span>
            <select
              className="input project-page-size"
              value={pageSize}
              onChange={(e) => setPageSize(Number(e.target.value) as (typeof PAGE_SIZES)[number])}
            >
              {PAGE_SIZES.map((size) => (
                <option key={size} value={size}>
                  {size} 条/页
                </option>
              ))}
            </select>
            <button
              type="button"
              className="btn-secondary text-sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
            >
              上一页
            </button>
            <span className="project-page-indicator">
              {Math.min(page, totalPages)} / {totalPages}
            </span>
            <button
              type="button"
              className="btn-secondary text-sm"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              下一页
            </button>
          </div>
        </>
      )}

      <ProjectCreateModal
        open={createOpen}
        workspaceId={workspaceId}
        onClose={() => {
          setCreateOpen(false);
          if (initialCreateOpen) router.replace(createQueryPath);
        }}
        onSuccess={handleCreateSuccess}
      />
    </div>
  );
}
