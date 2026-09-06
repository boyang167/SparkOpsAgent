"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ProjectCard } from "./project/ProjectCard";
import { ProjectCreateModal } from "./project/ProjectCreateModal";
import { ProjectConfigurePanel } from "./project/ProjectConfigurePanel";
import { ProjectReviewModal } from "./project/ProjectReviewModal";
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
import { projectConfigActionLabel, projectPrimaryAction, shouldShowEditConfigAction } from "./project/project-progress";

interface ProjectPanelProps {
  workspaceId: string;
  initialCreateOpen?: boolean;
  /** When create was opened via ?create=1, close clears this path instead of always /projects */
  createQueryPath?: string;
}

type ViewMode = "card" | "table";

const PAGE_SIZES = [6, 9, 12, 24] as const;

function SearchIcon() {
  return (
    <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path
        d="M9 3.5a5.5 5.5 0 1 1 0 11 5.5 5.5 0 0 1 0-11Z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path d="M13.5 13.5 17 17" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

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
  const [configureProject, setConfigureProject] = useState<ProjectVM | null>(null);
  const [reviewProject, setReviewProject] = useState<ProjectVM | null>(null);
  const [reviewLoading, setReviewLoading] = useState(false);
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

  const hasActiveFilters = Boolean(
    keyword.trim() || statusFilter || languageFilter || namespaceFilter.trim(),
  );

  useEffect(() => {
    setPage(1);
  }, [keyword, statusFilter, languageFilter, namespaceFilter, pageSize, viewMode]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = useMemo(() => {
    const current = Math.min(page, totalPages);
    const start = (current - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, page, pageSize, totalPages]);

  function resetFilters() {
    setKeyword("");
    setStatusFilter("");
    setLanguageFilter("");
    setNamespaceFilter("");
  }

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

  async function onSubmitReview(item: ProjectVM) {
    if (!window.confirm(`确定提交项目「${projectDisplayName(item)}」审核？提交后配置将暂时锁定。`)) return;
    setError(null);
    try {
      await workspace.submitProjectReview(workspaceId, item.id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "提交审核失败");
    }
  }

  async function onReview(input: Parameters<typeof workspace.reviewProject>[2]) {
    if (!reviewProject) return;
    setReviewLoading(true);
    setError(null);
    try {
      await workspace.reviewProject(workspaceId, reviewProject.id, input);
      setReviewProject(null);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "项目审核失败");
    } finally {
      setReviewLoading(false);
    }
  }

  function handleCreateSuccess() {
    void load();
  }

  return (
    <div className="projects-page">
      <section className="platform-table-card projects-card">
        <PlatformPanelHead
          title="项目管理"
          description="管理产品下的项目，创建、配置并进入项目工作区。"
          action={
            <button type="button" className="btn platform-add-btn" onClick={() => setCreateOpen(true)}>
              <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path d="M10 4v12M4 10h12" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
              </svg>
              创建项目
            </button>
          }
        />

        <div className="projects-toolbar">
          <div className="projects-toolbar-filters">
            <label className="projects-search-wrap">
              <SearchIcon />
              <input
                className="projects-search-input"
                placeholder="搜索项目名称或描述"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
            </label>
            <select
              className="projects-filter-select"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              aria-label="状态筛选"
            >
              <option value="">全部状态</option>
              {PROJECT_STATUS_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <select
              className="projects-filter-select"
              value={languageFilter}
              onChange={(e) => setLanguageFilter(e.target.value)}
              aria-label="开发语言筛选"
            >
              <option value="">全部语言</option>
              {PROJECT_LANGUAGE_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <input
              className="projects-filter-input"
              placeholder="命名空间"
              value={namespaceFilter}
              onChange={(e) => setNamespaceFilter(e.target.value)}
              aria-label="命名空间筛选"
            />
            {hasActiveFilters ? (
              <button type="button" className="btn-secondary projects-reset-btn" onClick={resetFilters}>
                重置
              </button>
            ) : null}
          </div>

          <div className="projects-toolbar-actions">
            <span className="projects-count">{filtered.length} 个项目</span>
            <div className="projects-view-toggle" role="group" aria-label="视图切换">
              <button
                type="button"
                className={viewMode === "card" ? "is-active" : ""}
                onClick={() => setViewMode("card")}
              >
                卡片
              </button>
              <button
                type="button"
                className={viewMode === "table" ? "is-active" : ""}
                onClick={() => setViewMode("table")}
              >
                表格
              </button>
            </div>
            <button
              type="button"
              className="btn-secondary projects-refresh-btn"
              onClick={() => void load()}
              disabled={loading}
              aria-label="刷新"
            >
              ↻
            </button>
          </div>
        </div>

        {error ? (
          <div className="projects-alert">
            <InlineAlert tone="error">{error}</InlineAlert>
          </div>
        ) : null}

        <div className="projects-body">
          {loading ? (
            <div className="projects-state">加载中…</div>
          ) : filtered.length === 0 ? (
            <div className="projects-state">
              <p>{hasActiveFilters ? "没有符合筛选条件的项目" : "暂无项目"}</p>
              {!hasActiveFilters ? (
                <button type="button" className="btn platform-add-btn" onClick={() => setCreateOpen(true)}>
                  创建第一个项目
                </button>
              ) : (
                <button type="button" className="btn-secondary" onClick={resetFilters}>
                  清除筛选
                </button>
              )}
            </div>
          ) : (
            <>
              {viewMode === "card" ? (
                <div className="project-card-grid">
                  {paged.map((project) => (
                    <ProjectCard
                      key={project.id}
                      project={project}
                      onArchive={(p) => void onArchive(p)}
                      onViewDetails={(p) => router.push(`/projects/${p.id}`)}
                      onContinueConfig={(p) => setConfigureProject(p)}
                      onSubmitReview={(p) => void onSubmitReview(p)}
                      onReview={(p) => setReviewProject(p)}
                      onDeploy={(p) => router.push(`/projects/${p.id}?release=1`)}
                    />
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
                            <Link href={`/projects/${project.id}`} className="project-table-name project-table-name-link">
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
                            </Link>
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
                              <button type="button" onClick={() => router.push(`/projects/${project.id}`)}>
                                查看详情
                              </button>
                              <Link href={`/projects/${project.id}`}>进入项目</Link>
                              {(() => {
                                const action = projectPrimaryAction(project);
                                if (action === "deploy") {
                                  return (
                                    <>
                                      {shouldShowEditConfigAction(project) ? (
                                        <button type="button" onClick={() => setConfigureProject(project)}>
                                          修改配置
                                        </button>
                                      ) : null}
                                      <button type="button" onClick={() => router.push(`/projects/${project.id}?release=1`)}>
                                        发布
                                      </button>
                                    </>
                                  );
                                }
                                if (action === "submit" || action === "review") {
                                  return (
                                    <>
                                      {shouldShowEditConfigAction(project) ? (
                                        <button type="button" onClick={() => setConfigureProject(project)}>
                                          修改配置
                                        </button>
                                      ) : null}
                                      {action === "submit" ? (
                                        <button type="button" onClick={() => void onSubmitReview(project)}>
                                          提交审核
                                        </button>
                                      ) : (
                                        <button type="button" onClick={() => setReviewProject(project)}>
                                          审核
                                        </button>
                                      )}
                                    </>
                                  );
                                }
                                if (action === "pending") {
                                  return (
                                    <button type="button" disabled>
                                      审核中
                                    </button>
                                  );
                                }
                                if (shouldShowEditConfigAction(project)) {
                                  return (
                                    <button type="button" onClick={() => setConfigureProject(project)}>
                                      {projectConfigActionLabel(project)}
                                    </button>
                                  );
                                }
                                return (
                                  <button type="button" onClick={() => setConfigureProject(project)}>
                                    继续配置
                                  </button>
                                );
                              })()}
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
        </div>
      </section>

      <ProjectCreateModal
        open={createOpen}
        workspaceId={workspaceId}
        onClose={() => {
          setCreateOpen(false);
          if (initialCreateOpen) router.replace(createQueryPath);
        }}
        onSuccess={handleCreateSuccess}
      />

      {configureProject ? (
        <ProjectConfigurePanel
          open={Boolean(configureProject)}
          workspaceId={workspaceId}
          project={configureProject}
          onClose={() => setConfigureProject(null)}
          onProjectUpdated={(updated) => setConfigureProject(updated)}
          onSaved={() => {
            setConfigureProject(null);
            void load();
          }}
        />
      ) : null}

      <ProjectReviewModal
        open={Boolean(reviewProject)}
        workspaceId={workspaceId}
        project={reviewProject}
        loading={reviewLoading}
        onClose={() => {
          if (!reviewLoading) setReviewProject(null);
        }}
        onSubmit={(input) => void onReview(input)}
      />
    </div>
  );
}
