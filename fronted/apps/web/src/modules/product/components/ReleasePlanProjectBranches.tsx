"use client";

import { useEffect, useMemo, useState } from "react";
import type { ProductProjectVM } from "@sparkops/contracts/product";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import { RunBranchField } from "./RunBranchField";
import type { GitRefKind, ProjectRef, ProjectRefOverrides } from "./release-plan-workflow-params";
import { peekWorkspaceRunAssets, prefetchWorkspaceRunAssets } from "./workflow-run-prefetch";

export interface ProjectRefsValue {
  projectIds: string[];
  refOverrides: ProjectRefOverrides;
}

interface ReleasePlanProjectBranchesProps {
  workspaceId: string;
  /** 产品下全部项目，用于显示名称与添加候选 */
  projects: ProductProjectVM[];
  selectedIds: string[];
  refOverrides: ProjectRefOverrides;
  /** 项目未单独指定时使用的分支 / Tag */
  defaultRefFor: (projectId: string) => ProjectRef;
  /** 工作流构建任务里为项目配置的仓库（优先于项目自身绑定的仓库） */
  workflowSourceFor?: (projectId: string) => { gitUrl: string; secretId: string } | undefined;
  /** 工作流含构建任务时才可添加项目、指定分支 */
  buildable: boolean;
  editable: boolean;
  /** 在其他 Modal 内使用时置为 true */
  nested?: boolean;
  onChange: (value: ProjectRefsValue) => void;
}

const SEARCH_THRESHOLD = 6;
const REF_KIND_LABEL: Record<GitRefKind, string> = { branch: "分支", tag: "Tag", pr: "PR" };

function projectLabel(project: ProductProjectVM | undefined, fallback: string): string {
  return project?.display_name || project?.name || fallback;
}

function matches(project: ProductProjectVM | undefined, projectId: string, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return [project?.display_name, project?.name, projectId].some((value) => value?.toLowerCase().includes(q));
}

function sameRef(a: ProjectRef, b: ProjectRef): boolean {
  return a.ref === b.ref && a.type === b.type;
}

export function ReleasePlanProjectBranches({
  workspaceId,
  projects,
  selectedIds,
  refOverrides,
  defaultRefFor,
  workflowSourceFor,
  buildable,
  editable,
  nested,
  onChange,
}: ReleasePlanProjectBranchesProps) {
  const [query, setQuery] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [batchOpen, setBatchOpen] = useState(false);
  const [gitProjects, setGitProjects] = useState<ProjectVM[]>(
    () => peekWorkspaceRunAssets(workspaceId)?.gitProjects ?? [],
  );

  useEffect(() => {
    if (!buildable || !workspaceId) return;
    let cancelled = false;
    void prefetchWorkspaceRunAssets(workspaceId)
      .then((assets) => {
        if (!cancelled) setGitProjects(assets.gitProjects);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [buildable, workspaceId]);

  const projectById = useMemo(() => new Map(projects.map((project) => [project.id, project])), [projects]);
  const gitById = useMemo(() => new Map(gitProjects.map((project) => [project.id, project])), [gitProjects]);
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);
  const candidates = useMemo(
    () => (buildable ? projects.filter((project) => !selectedSet.has(project.id)) : []),
    [buildable, projects, selectedSet],
  );
  const visibleIds = useMemo(
    () => selectedIds.filter((id) => matches(projectById.get(id), id, query)),
    [selectedIds, projectById, query],
  );
  const customCount = selectedIds.filter((id) => refOverrides[id]).length;
  const showSearch = selectedIds.length >= SEARCH_THRESHOLD;
  const canAdd = editable && candidates.length > 0;
  const canEditRef = editable && buildable;

  function sourceFor(projectId: string) {
    const fromWorkflow = workflowSourceFor?.(projectId);
    const bound = gitById.get(projectId);
    return {
      gitUrl: fromWorkflow?.gitUrl || bound?.git_url?.trim() || "",
      secretId: fromWorkflow?.secretId || bound?.secret_id?.trim() || "",
    };
  }

  function emit(projectIds: string[], overrides: ProjectRefOverrides) {
    const keep = new Set(projectIds);
    const next: ProjectRefOverrides = {};
    for (const [id, item] of Object.entries(overrides)) {
      if (keep.has(id) && item.ref.trim()) next[id] = { ref: item.ref.trim(), type: item.type };
    }
    onChange({ projectIds, refOverrides: next });
  }

  function withRef(overrides: ProjectRefOverrides, projectId: string, ref: ProjectRef | null) {
    const next = { ...overrides };
    if (!ref || !ref.ref.trim() || sameRef({ ref: ref.ref.trim(), type: ref.type }, defaultRefFor(projectId))) {
      delete next[projectId];
    } else {
      next[projectId] = ref;
    }
    return next;
  }

  function setRef(projectId: string, ref: ProjectRef | null) {
    emit(selectedIds, withRef(refOverrides, projectId, ref));
  }

  function remove(projectId: string) {
    if (selectedIds.length <= 1) return;
    emit(
      selectedIds.filter((id) => id !== projectId),
      refOverrides,
    );
  }

  function add(ids: string[]) {
    emit([...selectedIds, ...ids.filter((id) => !selectedSet.has(id))], refOverrides);
    setAddOpen(false);
  }

  function applyBatch(ref: ProjectRef | null) {
    let next = refOverrides;
    for (const id of visibleIds) next = withRef(next, id, ref);
    emit(selectedIds, next);
    setBatchOpen(false);
  }

  if (selectedIds.length === 0 && !canAdd) return null;

  return (
    <div className="release-plan-projects" role="group" aria-label="本次执行项目">
      <div className="release-plan-projects-toolbar">
        {showSearch ? (
          <input
            type="search"
            className="input text-xs release-plan-projects-search"
            placeholder="搜索项目"
            aria-label="搜索执行项目"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        ) : null}
        <span className="release-plan-projects-summary">
          {selectedIds.length} 个项目
          {customCount > 0 ? ` · ${customCount} 个单独指定` : ""}
        </span>
        <span className="release-plan-projects-actions">
          {canEditRef && selectedIds.length > 1 ? (
            <button type="button" className="release-plan-projects-action" onClick={() => setBatchOpen(true)}>
              {query.trim() ? "批量设置筛选结果" : "批量设置"}
            </button>
          ) : null}
          {canAdd ? (
            <button type="button" className="release-plan-projects-action" onClick={() => setAddOpen(true)}>
              + 添加项目
            </button>
          ) : null}
        </span>
      </div>

      {selectedIds.length > 0 ? (
        <ul className="release-plan-project-rows">
          {visibleIds.length === 0 ? (
            <li className="release-plan-project-rows-empty">没有匹配「{query.trim()}」的项目</li>
          ) : (
            visibleIds.map((projectId) => {
              const label = projectLabel(projectById.get(projectId), projectId);
              const override = refOverrides[projectId];
              const effective = override ?? defaultRefFor(projectId);
              const source = sourceFor(projectId);
              return (
                <li key={projectId} className={`release-plan-project-row${override ? " is-custom" : ""}`}>
                  <span className="release-plan-project-row-name" title={label}>
                    {label}
                  </span>
                  {buildable ? (
                    <span className="release-plan-project-row-ref">
                      {canEditRef ? (
                        <RunBranchField
                          compact
                          workspaceId={workspaceId}
                          gitUrl={source.gitUrl}
                          secretId={source.secretId}
                          value={effective.ref}
                          refType={effective.type}
                          label={`${label} 构建分支或 Tag`}
                          onChange={(ref, type) => setRef(projectId, { ref, type })}
                        />
                      ) : (
                        <span className="release-plan-project-row-ref-text">
                          {effective.type === "pr" ? `#${effective.ref}` : effective.ref}
                        </span>
                      )}
                      {effective.type !== "branch" ? (
                        <span className="release-plan-project-row-kind">{REF_KIND_LABEL[effective.type]}</span>
                      ) : null}
                    </span>
                  ) : null}
                  <span className="release-plan-project-row-actions">
                    {canEditRef && override ? (
                      <button
                        type="button"
                        className="release-plan-project-row-reset"
                        title={`恢复为默认（${defaultRefFor(projectId).ref}）`}
                        onClick={() => setRef(projectId, null)}
                      >
                        恢复默认
                      </button>
                    ) : null}
                    {editable ? (
                      <button
                        type="button"
                        className="release-plan-project-row-remove"
                        aria-label={`移除 ${label}`}
                        title={selectedIds.length <= 1 ? "至少保留一个项目" : "移除"}
                        disabled={selectedIds.length <= 1}
                        onClick={() => remove(projectId)}
                      >
                        ×
                      </button>
                    ) : null}
                  </span>
                </li>
              );
            })
          )}
        </ul>
      ) : null}

      {addOpen ? (
        <AddProjectsModal candidates={candidates} nested={nested} onClose={() => setAddOpen(false)} onSubmit={add} />
      ) : null}
      {batchOpen ? (
        <BatchRefModal
          count={visibleIds.length}
          nested={nested}
          onClose={() => setBatchOpen(false)}
          onSubmit={applyBatch}
        />
      ) : null}
    </div>
  );
}

function AddProjectsModal({
  candidates,
  nested,
  onClose,
  onSubmit,
}: {
  candidates: ProductProjectVM[];
  nested?: boolean;
  onClose: () => void;
  onSubmit: (projectIds: string[]) => void;
}) {
  const [query, setQuery] = useState("");
  const [picked, setPicked] = useState<Set<string>>(() => new Set());
  const visible = candidates.filter((project) => matches(project, project.id, query));
  const allVisiblePicked = visible.length > 0 && visible.every((project) => picked.has(project.id));

  function toggle(projectId: string) {
    setPicked((current) => {
      const next = new Set(current);
      if (next.has(projectId)) next.delete(projectId);
      else next.add(projectId);
      return next;
    });
  }

  function toggleAllVisible() {
    setPicked((current) => {
      const next = new Set(current);
      for (const project of visible) {
        if (allVisiblePicked) next.delete(project.id);
        else next.add(project.id);
      }
      return next;
    });
  }

  return (
    <Modal
      open
      nested={nested}
      onClose={onClose}
      title="添加执行项目"
      description="新加入的项目使用默认分支，添加后可逐个修改。"
      footer={
        <ReleasePlanModalFooter
          onCancel={onClose}
          submitLabel={picked.size > 0 ? `添加 ${picked.size} 个项目` : "添加"}
          disabled={picked.size === 0}
          onSubmit={() => onSubmit(candidates.filter((project) => picked.has(project.id)).map((project) => project.id))}
        />
      }
    >
      <div className="stack-sm">
        <input
          type="search"
          className="input text-sm"
          placeholder="搜索项目名称"
          aria-label="搜索项目"
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <div className="release-plan-pick-head">
          <button
            type="button"
            className="release-plan-projects-action"
            disabled={visible.length === 0}
            onClick={toggleAllVisible}
          >
            {allVisiblePicked ? "取消全选" : query.trim() ? "全选筛选结果" : "全选"}
          </button>
          <span className="release-plan-projects-summary">已选 {picked.size} 个</span>
        </div>
        <ul className="release-plan-pick-list">
          {visible.length === 0 ? (
            <li className="release-plan-project-rows-empty">
              {candidates.length === 0 ? "产品下的项目都已加入" : `没有匹配「${query.trim()}」的项目`}
            </li>
          ) : (
            visible.map((project) => (
              <li key={project.id}>
                <label className="release-plan-pick-item">
                  <input type="checkbox" checked={picked.has(project.id)} onChange={() => toggle(project.id)} />
                  <span className="release-plan-pick-name">{projectLabel(project, project.id)}</span>
                  {project.display_name && project.name && project.display_name !== project.name ? (
                    <span className="release-plan-pick-sub">{project.name}</span>
                  ) : null}
                </label>
              </li>
            ))
          )}
        </ul>
      </div>
    </Modal>
  );
}

function BatchRefModal({
  count,
  nested,
  onClose,
  onSubmit,
}: {
  count: number;
  nested?: boolean;
  onClose: () => void;
  onSubmit: (ref: ProjectRef | null) => void;
}) {
  const [type, setType] = useState<"branch" | "tag">("branch");
  const [ref, setRef] = useState("");
  const submit = () => onSubmit(ref.trim() ? { ref: ref.trim(), type } : null);

  return (
    <Modal
      open
      size="sm"
      nested={nested}
      onClose={onClose}
      title="批量设置分支 / Tag"
      description={`应用到当前列表中的 ${count} 个项目。留空则全部恢复为默认分支。`}
      footer={<ReleasePlanModalFooter onCancel={onClose} submitLabel="应用" onSubmit={submit} />}
    >
      <div className="stack-sm">
        <div className="release-plan-ref-kind" role="radiogroup" aria-label="引用类型">
          {(["branch", "tag"] as const).map((kind) => (
            <button
              key={kind}
              type="button"
              role="radio"
              aria-checked={type === kind}
              className={`release-plan-ref-kind-option${type === kind ? " is-active" : ""}`}
              onClick={() => setType(kind)}
            >
              {REF_KIND_LABEL[kind]}
            </button>
          ))}
        </div>
        <input
          className="input font-mono text-sm"
          placeholder={type === "tag" ? "例如 v1.2.0" : "例如 release/1.2"}
          aria-label={type === "tag" ? "Tag 名称" : "分支名称"}
          autoFocus
          spellCheck={false}
          value={ref}
          onChange={(e) => setRef(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit();
          }}
        />
      </div>
    </Modal>
  );
}
