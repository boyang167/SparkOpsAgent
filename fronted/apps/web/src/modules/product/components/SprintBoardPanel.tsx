"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  PmProvider,
  ProductWorkflowVM,
  SprintBoardColumnVM,
  SprintBoardVM,
  SprintMetricsVM,
  SprintSummaryVM,
  WorkspaceSprintProcessTemplateVM,
  SprintStage,
  WorkItemCardVM,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { useSprintBoardSync } from "@/modules/product/hooks/useSprintBoardSync";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail } from "./ProductMasterDetail";
import { ReleasePlansPanel } from "./ReleasePlansPanel";
import { SprintStageEditor } from "./SprintStageEditor";
import { buildStageMigrations } from "./sprint-stage-migrations";
import { isValidStageKey, normalizeStageForSave, withSortOrder } from "./sprint-process-templates";
import { SprintWorkItemExecuteModal } from "./SprintWorkItemExecuteModal";
import { PM_PROVIDER_LABEL } from "./pm-labels";
import { WorkItemCardMenu } from "./WorkItemCardMenu";
import { WorkItemDetailDrawer } from "./WorkItemDetailDrawer";
import { WorkItemPmChip } from "./WorkItemPmChip";
import { WorkItemPmLinkModal } from "./WorkItemPmLinkModal";

interface SprintBoardPanelProps {
  workspaceId: string;
  productId: string;
}

const STATUS_LABEL: Record<string, string> = {
  planning: "待启动",
  active: "进行中",
  completed: "已完成",
  archived: "已归档",
};

const PRIORITY_LABEL: Record<string, string> = {
  critical: "紧急",
  high: "高",
  medium: "中",
  low: "低",
};

const RUN_STATUS_LABEL: Record<string, string> = {
  running: "运行中",
  success: "已成功",
  failed: "已失败",
};

function runStatusClass(status?: string) {
  switch (status) {
    case "running":
      return "sprint-run-status is-running";
    case "success":
      return "sprint-run-status is-success";
    case "failed":
      return "sprint-run-status is-failed";
    default:
      return "";
  }
}

function statusTone(status: string) {
  switch (status) {
    case "active":
      return "text-primary";
    case "completed":
      return "text-emerald-600";
    case "archived":
      return "text-muted-foreground";
    default:
      return "text-amber-600";
  }
}

export function SprintBoardPanel({ workspaceId, productId }: SprintBoardPanelProps) {
  const [sprints, setSprints] = useState<SprintSummaryVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [board, setBoard] = useState<SprintBoardVM | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [boardLoading, setBoardLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [viewTab, setViewTab] = useState<"board" | "release" | "metrics">("board");
  const [metrics, setMetrics] = useState<SprintMetricsVM | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createGoal, setCreateGoal] = useState("");
  const [createTemplateId, setCreateTemplateId] = useState("");
  const [wsProcessTemplates, setWsProcessTemplates] = useState<WorkspaceSprintProcessTemplateVM[]>([]);
  const [createTemplatesLoading, setCreateTemplatesLoading] = useState(false);
  const [stagesEditOpen, setStagesEditOpen] = useState(false);
  const [editStages, setEditStages] = useState<SprintStage[]>([]);
  const [itemOpen, setItemOpen] = useState(false);
  const [itemTitle, setItemTitle] = useState("");
  const [itemStageId, setItemStageId] = useState("");
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [pmImportOpen, setPmImportOpen] = useState(false);
  const [pmProvider, setPmProvider] = useState<PmProvider>("jira");
  const [pmWorkspaceId, setPmWorkspaceId] = useState("");
  const [pmIterationId, setPmIterationId] = useState("");
  const [pmSyncMode, setPmSyncMode] = useState<"link_only" | "bidirectional">("link_only");
  const [pmLinkTarget, setPmLinkTarget] = useState<WorkItemCardVM | null>(null);
  const [workItemDrawerOpen, setWorkItemDrawerOpen] = useState(false);
  const [selectedWorkItem, setSelectedWorkItem] = useState<WorkItemCardVM | null>(null);
  const [executeModalOpen, setExecuteModalOpen] = useState(false);
  const [executeContext, setExecuteContext] = useState<{
    item: WorkItemCardVM;
    stage: SprintStage;
    workflowId: string;
  } | null>(null);
  const [cardMenuOpenId, setCardMenuOpenId] = useState<string | null>(null);
  const [blockingPlans, setBlockingPlans] = useState<Array<{ id: string; name: string; status: string }>>([]);
  const [activeReleasePlanCount, setActiveReleasePlanCount] = useState(0);

  const loadReleasePlansMeta = useCallback(
    async (sprintId: string) => {
      try {
        const plans = await product.listReleasePlans(workspaceId, productId, sprintId);
        setActiveReleasePlanCount(
          plans.filter((p) => ["planning", "wait_for_approval", "executing"].includes(p.status)).length,
        );
        setBlockingPlans(
          plans
            .filter((p) => ["executing", "wait_for_approval"].includes(p.status))
            .map((p) => ({ id: p.id, name: p.name, status: p.status })),
        );
      } catch {
        setActiveReleasePlanCount(0);
        setBlockingPlans([]);
      }
    },
    [workspaceId, productId],
  );

  const loadSprints = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await product.listSprints(workspaceId, productId);
      setSprints(items);
      if (items.length > 0) {
        setSelectedId((prev) => prev ?? items[0].id);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载迭代列表失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  const loadBoard = useCallback(async () => {
    if (!selectedId) {
      setBoard(null);
      return;
    }
    setBoardLoading(true);
    setError(null);
    try {
      setBoard(await product.getSprintBoard(workspaceId, productId, selectedId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载看板失败");
      setBoard(null);
    } finally {
      setBoardLoading(false);
    }
  }, [workspaceId, productId, selectedId]);

  useEffect(() => {
    void loadSprints();
  }, [loadSprints]);

  useEffect(() => {
    void loadBoard();
  }, [loadBoard]);

  useSprintBoardSync({
    workspaceId,
    productId,
    sprintId: selectedId,
    enabled: viewTab === "board" && Boolean(selectedId) && !workItemDrawerOpen && !executeModalOpen,
    onBoardChanged: () => {
      if (workItemDrawerOpen || executeModalOpen) return;
      void loadBoard();
    },
  });

  useEffect(() => {
    setViewTab("board");
    setMetrics(null);
    if (selectedId) void loadReleasePlansMeta(selectedId);
  }, [selectedId, loadReleasePlansMeta]);

  const loadMetrics = useCallback(async () => {
    if (!selectedId) {
      setMetrics(null);
      return;
    }
    setMetricsLoading(true);
    try {
      setMetrics(await product.getSprintMetrics(workspaceId, productId, selectedId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载度量失败");
      setMetrics(null);
    } finally {
      setMetricsLoading(false);
    }
  }, [workspaceId, productId, selectedId]);

  useEffect(() => {
    if (viewTab === "metrics" && selectedId) {
      void loadMetrics();
    }
  }, [viewTab, selectedId, loadMetrics]);

  useEffect(() => {
    void product.listProductWorkflows(workspaceId, productId).then(setWorkflows).catch(() => setWorkflows([]));
  }, [workspaceId, productId]);

  useEffect(() => {
    if (!createOpen) return;
    let cancelled = false;
    setCreateTemplatesLoading(true);
    void product
      .listWorkspaceSprintProcessTemplates(workspaceId)
      .then((items) => {
        if (cancelled) return;
        setWsProcessTemplates(items);
        const defaultTpl = items.find((item) => item.template_key === "agile-4") ?? items[0];
        setCreateTemplateId(defaultTpl?.id ?? "");
      })
      .catch(() => {
        if (!cancelled) {
          setWsProcessTemplates([]);
          setCreateTemplateId("");
        }
      })
      .finally(() => {
        if (!cancelled) setCreateTemplatesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [createOpen, workspaceId]);

  useEffect(() => {
    if (!cardMenuOpenId) return;
    const onDocClick = () => setCardMenuOpenId(null);
    document.addEventListener("click", onDocClick);
    return () => document.removeEventListener("click", onDocClick);
  }, [cardMenuOpenId]);

  const hasRunningItems = useMemo(() => {
    if (!board) return false;
    return board.columns.some((col) =>
      col.work_items.some((item) => item.delivery?.last_workflow_status === "running"),
    );
  }, [board]);

  const boardStages = useMemo(() => {
    const fromSprint = board?.sprint.stages;
    if (fromSprint && fromSprint.length > 0) return fromSprint;
    return board?.columns.map((column) => column.stage) ?? [];
  }, [board?.sprint.stages, board?.columns]);

  const boardWorkItems = useMemo(
    () => board?.columns.flatMap((column) => column.work_items) ?? [],
    [board?.columns],
  );

  useEffect(() => {
    if (!hasRunningItems || !selectedId || workItemDrawerOpen || executeModalOpen) return;
    const timer = window.setInterval(() => {
      void loadBoard();
    }, 4000);
    return () => window.clearInterval(timer);
  }, [hasRunningItems, selectedId, loadBoard, workItemDrawerOpen, executeModalOpen]);

  const filteredSprints = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return sprints;
    return sprints.filter((s) => s.name.toLowerCase().includes(q));
  }, [sprints, searchQuery]);

  const listItems = useMemo(
    () =>
      filteredSprints.map((row) => ({
        id: row.id,
        title: row.name,
        meta: STATUS_LABEL[row.status] ?? row.status,
        subtitle: row.goal,
      })),
    [filteredSprints],
  );

  async function resolveProcessIdForTemplate(templateId: string): Promise<string | undefined> {
    if (!templateId) return undefined;
    const tpl = wsProcessTemplates.find((item) => item.id === templateId);
    if (!tpl) return undefined;
    const processes = await product.listSprintProcesses(workspaceId, productId);
    if (tpl.template_key) {
      const existing = processes.find((proc) => proc.template_key === tpl.template_key);
      if (existing) return existing.id;
    }
    const imported = await product.importWorkspaceSprintTemplate(workspaceId, productId, {
      workspace_template_id: tpl.id,
      name: tpl.name,
    });
    return imported.id;
  }

  async function onCreateSprint() {
    if (!createName.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const processId = createTemplateId ? await resolveProcessIdForTemplate(createTemplateId) : undefined;
      const created = await product.createSprint(workspaceId, productId, {
        name: createName.trim(),
        goal: createGoal.trim() || undefined,
        ...(processId ? { process_id: processId } : {}),
      });
      setCreateOpen(false);
      setCreateName("");
      setCreateGoal("");
      setCreateTemplateId("");
      await loadSprints();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建迭代失败");
    } finally {
      setBusy(false);
    }
  }

  function openEditStages() {
    const sourceStages =
      board?.sprint.stages && board.sprint.stages.length > 0
        ? board.sprint.stages
        : board?.columns.map((column) => column.stage) ?? [];
    if (sourceStages.length === 0) return;
    setError(null);
    setEditStages(sourceStages.map((stage) => ({ ...stage })));
    setStagesEditOpen(true);
  }

  async function onSaveSprintStages() {
    if (!selectedId || !board) return;
    const normalizedStages = withSortOrder(editStages.map((stage, index) => normalizeStageForSave(stage, index)));
    const invalidStage = normalizedStages.find((stage) => !stage.name || !isValidStageKey(stage.key));
    if (invalidStage) {
      setError("阶段名称不能为空，且标识 key 须以小写字母开头（如 dev、stage_5）");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const stageMigrations = buildStageMigrations(board.sprint.stages ?? [], normalizedStages, board);
      await product.updateSprint(workspaceId, productId, selectedId, {
        stages: normalizedStages,
        stage_migrations: Object.keys(stageMigrations).length > 0 ? stageMigrations : undefined,
      });
      setStagesEditOpen(false);
      await loadBoard();
      await loadSprints();
    } catch (err) {
      const msg =
        err instanceof BffError && err.code === "STAGE_HAS_WORK_ITEMS"
          ? "删除的阶段仍有工作项，请先将工作项移到其他列"
          : err instanceof BffError && err.code === "SPRINT_PROCESS_INVALID"
            ? "阶段配置无效：名称不能为空，key 须以小写字母开头且唯一（如 dev、stage_5）"
            : err instanceof BffError
              ? err.message
              : "保存看板列失败";
      setError(msg);
    } finally {
      setBusy(false);
    }
  }

  async function onStartSprint() {
    if (!selectedId) return;
    setBusy(true);
    try {
      await product.startSprint(workspaceId, productId, selectedId);
      await loadSprints();
      await loadBoard();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "启动迭代失败");
    } finally {
      setBusy(false);
    }
  }

  async function onCompleteSprint() {
    if (!selectedId) return;
    setBusy(true);
    setBlockingPlans([]);
    try {
      await product.completeSprint(workspaceId, productId, selectedId);
      await loadSprints();
      await loadBoard();
      await loadReleasePlansMeta(selectedId);
    } catch (err) {
      if (err instanceof BffError && err.code === "SPRINT_BLOCKING_RELEASE") {
        await loadReleasePlansMeta(selectedId);
        setError("存在未完成的发布计划，请先完成或取消后再结束迭代");
      } else {
        setError(err instanceof BffError ? err.message : "完成迭代失败");
      }
    } finally {
      setBusy(false);
    }
  }

  async function onCreateWorkItem() {
    if (!selectedId || !itemTitle.trim() || !itemStageId) return;
    setBusy(true);
    try {
      await product.createWorkItem(workspaceId, productId, selectedId, {
        stage_id: itemStageId,
        title: itemTitle.trim(),
        type: "story",
        priority: "medium",
      });
      setItemOpen(false);
      setItemTitle("");
      await loadBoard();
      await loadSprints();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建工作项失败");
    } finally {
      setBusy(false);
    }
  }

  async function bindStageWorkflow(stageId: string, workflowId: string) {
    if (!board || !selectedId) return;
    const stages: SprintStage[] = board.sprint.stages.map((stage) => {
      if (stage.id !== stageId) return stage;
      return {
        ...stage,
        workflows: workflowId
          ? [{ workflow_id: workflowId, trigger_mode: "manual" as const, default_parameters: {}, required_for_exit: false }]
          : [],
      };
    });
    setBusy(true);
    try {
      await product.updateSprint(workspaceId, productId, selectedId, { stages });
      await loadBoard();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "绑定工作流失败");
    } finally {
      setBusy(false);
    }
  }

  function openWorkItemDetail(item: WorkItemCardVM) {
    setSelectedWorkItem(item);
    setWorkItemDrawerOpen(true);
  }

  function openExecuteWorkflow(item: WorkItemCardVM, stage: SprintStage, workflowId?: string) {
    const wfId = workflowId ?? stage.workflows?.[0]?.workflow_id;
    if (!wfId) return;
    setExecuteContext({ item, stage, workflowId: wfId });
    setExecuteModalOpen(true);
  }

  function openCreateItem(stageId: string) {
    setItemStageId(stageId);
    setItemTitle("");
    setItemOpen(true);
  }

  const createItemStageName = useMemo(() => {
    if (!itemStageId || !board) return "";
    return board.columns.find((col) => col.stage.id === itemStageId)?.stage.name ?? "";
  }, [itemStageId, board]);

  async function onImportPm() {
    if (!selectedId || !pmIterationId.trim()) return;
    if (pmProvider === "tapd" && !pmWorkspaceId.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const result = await product.importPmSprint(workspaceId, productId, selectedId, {
        provider: pmProvider,
        pm_workspace_id: pmWorkspaceId.trim() || undefined,
        iteration_id: pmIterationId.trim(),
        sync_mode: pmSyncMode,
      });
      setPmImportOpen(false);
      setPmIterationId("");
      setPmWorkspaceId("");
      await loadBoard();
      await loadSprints();
      if (result.imported_count === 0 && result.skipped_count > 0) {
        setError(`未导入新工作项，${result.skipped_count} 条 Issue 已存在`);
      }
    } catch (err) {
      const msg =
        err instanceof BffError && err.code === "PM_NOT_CONFIGURED"
          ? `工作区未配置 ${PM_PROVIDER_LABEL[pmProvider]} 凭据（需在「密钥」中添加 service_type=${pmProvider}）`
          : err instanceof BffError
            ? err.message
            : "PM 导入失败";
      setError(msg);
    } finally {
      setBusy(false);
    }
  }

  function renderCard(item: WorkItemCardVM, _columns: SprintBoardColumnVM[]) {
    const runStatus = item.delivery?.last_workflow_status;
    const canManageCard =
      board?.sprint.status !== "archived" && board?.sprint.status !== "completed";
    return (
      <article key={item.id} className="sprint-board-card">
        <div className="sprint-board-card-top">
          <div className="sprint-board-card-head">
            <span className="sprint-board-card-type">{item.type}</span>
            <span className="sprint-board-card-priority">{PRIORITY_LABEL[item.priority] ?? item.priority}</span>
            {runStatus ? (
              <span className={runStatusClass(runStatus)}>{RUN_STATUS_LABEL[runStatus] ?? runStatus}</span>
            ) : null}
          </div>
          {canManageCard ? (
            <WorkItemCardMenu
              item={item}
              stages={boardStages}
              workspaceId={workspaceId}
              productId={productId}
              sprintActive={board?.sprint.status === "active"}
              disabled={busy}
              open={cardMenuOpenId === item.id}
              onToggle={() => setCardMenuOpenId((prev) => (prev === item.id ? null : item.id))}
              onClose={() => setCardMenuOpenId(null)}
              onChanged={() => {
                void loadBoard();
                void loadSprints();
              }}
            />
          ) : null}
        </div>
        <button
          type="button"
          className="sprint-board-card-title sprint-board-card-title-btn"
          onClick={() => openWorkItemDetail(item)}
        >
          {item.title}
        </button>
        {canManageCard || item.external?.issue_key ? (
          <WorkItemPmChip
            external={item.external}
            disabled={busy}
            compact
            onManage={() => setPmLinkTarget(item)}
            onLink={canManageCard ? () => setPmLinkTarget(item) : undefined}
          />
        ) : null}
      </article>
    );
  }

  const sprint = board?.sprint;

  return (
    <>
      {error ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      {blockingPlans.length > 0 ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="warning">
            <div className="stack-xs">
              <span>以下发布计划阻塞迭代完成：</span>
              <ul className="list-disc pl-4">
                {blockingPlans.map((plan) => (
                  <li key={plan.id}>
                    <button
                      type="button"
                      className="text-primary hover:underline"
                      onClick={() => setViewTab("release")}
                    >
                      {plan.name} ({plan.status})
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle="迭代"
        searchPlaceholder="搜索迭代"
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        items={listItems}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onAdd={() => setCreateOpen(true)}
        addAriaLabel="新建迭代"
        loading={loading}
        showMain={Boolean(selectedId)}
        emptyMain={
          <ProductDetailEmptyState
            title="暂无迭代，请点击 + 新建"
            description="创建迭代后可按阶段管理工作项，串联开发与发布流程。"
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={() => setCreateOpen(true)}>
                新建迭代
              </button>
            }
          />
        }
      >
        {boardLoading ? (
          <div className="product-master-main-inner">
            <div className="empty-state" role="status">
              看板加载中…
            </div>
          </div>
        ) : sprint ? (
          <>
            <div className="product-master-main-inner stack-md">
              <div className="flex flex-wrap justify-between items-start gap-3">
                <div className="stack-xs">
                  <h2 className="section-title">{sprint.name}</h2>
                  {sprint.goal ? <p className="text-sm text-muted-foreground">{sprint.goal}</p> : null}
                  <p className="text-xs text-muted-foreground">
                    工作项 {sprint.work_item_count ?? 0} · 已完成 {sprint.done_work_item_count ?? 0}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <span className={`text-sm font-medium ${statusTone(sprint.status)}`}>
                    {STATUS_LABEL[sprint.status] ?? sprint.status}
                  </span>
                  {sprint.status === "planning" || sprint.status === "active" ? (
                    <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => setPmImportOpen(true)}>
                      导入 PM
                    </button>
                  ) : null}
                  {sprint.status === "planning" ? (
                    <>
                      <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={openEditStages}>
                        编辑看板列
                      </button>
                      <button type="button" className="btn btn-primary text-xs" disabled={busy} onClick={() => void onStartSprint()}>
                        启动迭代
                      </button>
                    </>
                  ) : null}
                  {sprint.status === "active" ? (
                    <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onCompleteSprint()}>
                      完成迭代
                    </button>
                  ) : null}
                </div>
              </div>

              <div className="flex gap-2 text-sm border-b pb-2">
                <button
                  type="button"
                  className={viewTab === "board" ? "font-medium text-primary" : "text-muted-foreground"}
                  onClick={() => setViewTab("board")}
                >
                  看板
                </button>
                <button
                  type="button"
                  className={viewTab === "release" ? "font-medium text-primary" : "text-muted-foreground"}
                  onClick={() => setViewTab("release")}
                >
                  发布计划
                  {activeReleasePlanCount > 0 ? (
                    <span className="ml-1 rounded-full bg-primary/10 text-primary px-1.5 text-xs">{activeReleasePlanCount}</span>
                  ) : null}
                </button>
                <button
                  type="button"
                  className={viewTab === "metrics" ? "font-medium text-primary" : "text-muted-foreground"}
                  onClick={() => setViewTab("metrics")}
                >
                  度量
                </button>
              </div>
            </div>

            {viewTab === "release" && selectedId ? (
              <ReleasePlansPanel workspaceId={workspaceId} productId={productId} sprintId={selectedId} embedded />
            ) : viewTab === "metrics" ? (
              <div className="product-master-main-inner stack-md">
                {metricsLoading ? (
                  <p className="text-sm text-muted-foreground">度量加载中…</p>
                ) : metrics ? (
                  <>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                      <div className="rounded border p-3 stack-xs">
                        <span className="text-xs text-muted-foreground">工作项总数</span>
                        <span className="text-xl font-semibold">{metrics.total_work_items}</span>
                      </div>
                      <div className="rounded border p-3 stack-xs">
                        <span className="text-xs text-muted-foreground">已完成</span>
                        <span className="text-xl font-semibold text-emerald-600">{metrics.done_work_items}</span>
                      </div>
                      <div className="rounded border p-3 stack-xs">
                        <span className="text-xs text-muted-foreground">阻塞中</span>
                        <span className="text-xl font-semibold text-amber-600">{metrics.blocked_work_items}</span>
                      </div>
                      <div className="rounded border p-3 stack-xs">
                        <span className="text-xs text-muted-foreground">工作流执行次数</span>
                        <span className="text-xl font-semibold">{metrics.deployment_frequency ?? 0}</span>
                      </div>
                    </div>
                    {metrics.change_failure_rate != null ? (
                      <p className="text-sm text-muted-foreground">
                        变更失败率：{(metrics.change_failure_rate * 100).toFixed(1)}%
                      </p>
                    ) : null}
                    {metrics.burndown && metrics.burndown.length > 0 ? (
                      <div className="stack-xs">
                        <h3 className="text-sm font-medium">燃尽趋势（剩余工作项）</h3>
                        <ul className="text-xs grid grid-cols-2 md:grid-cols-4 gap-2">
                          {metrics.burndown.slice(-8).map((point) => (
                            <li key={point.date} className="border rounded px-2 py-1">
                              {point.date}：<strong>{point.remaining}</strong>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ) : null}
                  </>
                ) : (
                  <p className="text-sm text-muted-foreground">暂无度量数据</p>
                )}
              </div>
            ) : (
              <div className="product-master-main-inner sprint-board-wrap">
                <div className="sprint-board" role="region" aria-label="迭代看板">
                  {board.columns.map((column) => (
                    <section key={column.stage.id} className="sprint-board-column">
                      <header className="sprint-board-column-head" style={{ borderTopColor: column.stage.color ?? "var(--primary)" }}>
                        <div className="stack-xs w-full min-w-0">
                          <div className="flex items-center justify-between gap-2">
                            <span className="sprint-board-column-title">{column.stage.name}</span>
                        <span className={`sprint-board-column-count ${column.wip_limit_exceeded ? "text-red-600" : ""}`}>
                          {column.wip_count}
                          {column.stage.wip_limit ? ` / ${column.stage.wip_limit}` : ""}
                        </span>
                      </div>
                      {column.wip_limit_exceeded ? (
                        <span className="text-xs text-red-600">WIP 超限</span>
                      ) : null}
                          {sprint.status !== "archived" ? (
                            <select
                              className="input text-xs w-full"
                              value={column.stage.workflows?.[0]?.workflow_id ?? ""}
                              disabled={busy}
                              onChange={(e) => void bindStageWorkflow(column.stage.id, e.target.value)}
                              aria-label={`${column.stage.name} 绑定工作流`}
                            >
                              <option value="">绑定工作流…</option>
                              {workflows.map((wf) => (
                                <option key={wf.id} value={wf.id}>
                                  {wf.name}
                                </option>
                              ))}
                            </select>
                          ) : null}
                        </div>
                      </header>
                      <div className="sprint-board-column-body">
                        {column.work_items.length === 0 ? (
                          <p className="sprint-board-column-empty">拖拽或点击下方添加</p>
                        ) : (
                          column.work_items.map((item) => renderCard(item, board.columns))
                        )}
                      </div>
                      {sprint.status === "planning" || sprint.status === "active" ? (
                        <div className="sprint-board-column-footer">
                          <button
                            type="button"
                            className="sprint-board-column-add"
                            disabled={busy}
                            onClick={() => openCreateItem(column.stage.id)}
                          >
                            + 添加工作项
                          </button>
                        </div>
                      ) : null}
                    </section>
                  ))}
                </div>
              </div>
            )}
          </>
        ) : null}
      </ProductMasterDetail>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="新建迭代"
        description="为产品创建一次交付周期，例如 v3.2.4。"
        footer={
          <>
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy || !createName.trim()} onClick={() => void onCreateSprint()}>
              {busy ? "创建中…" : "创建"}
            </button>
          </>
        }
      >
        <div className="stack-sm">
          <label className="stack-xs">
            <span className="text-sm">迭代名称</span>
            <input className="input" value={createName} onChange={(e) => setCreateName(e.target.value)} placeholder="例如 v3.2.4" />
          </label>
          <label className="stack-xs">
            <span className="text-sm">迭代目标</span>
            <textarea className="input min-h-20" value={createGoal} onChange={(e) => setCreateGoal(e.target.value)} placeholder="可选：本次迭代要达成的目标" />
          </label>
          <label className="stack-xs">
            <span className="text-sm">迭代流程模板</span>
            <select
              className="input"
              value={createTemplateId}
              disabled={createTemplatesLoading || wsProcessTemplates.length === 0}
              onChange={(e) => setCreateTemplateId(e.target.value)}
            >
              {createTemplatesLoading ? <option value="">加载模板…</option> : null}
              {!createTemplatesLoading && wsProcessTemplates.length === 0 ? (
                <option value="">暂无工作区模板</option>
              ) : null}
              {wsProcessTemplates.map((tpl) => (
                <option key={tpl.id} value={tpl.id}>
                  {tpl.name} · {tpl.stages?.length ?? 0} 列
                  {tpl.template_key === "agile-4" ? "（默认）" : ""}
                </option>
              ))}
            </select>
            <span className="text-xs text-muted-foreground">
              模板来自工作空间，
              <Link href="/templates?tab=sprint-process" className="link-primary">前往模板管理</Link>
            </span>
          </label>
        </div>
      </Modal>

      <Modal
        open={stagesEditOpen}
        onClose={() => setStagesEditOpen(false)}
        title="编辑看板列"
        description="调整列名称、顺序与颜色。删除有卡片的列时，工作项会迁到第一列。"
        size="lg"
        panelClassName="sprint-stage-edit-modal"
        footer={
          <>
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setStagesEditOpen(false)}>取消</button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy || editStages.length < 2} onClick={() => void onSaveSprintStages()}>
              {busy ? "保存中…" : "保存"}
            </button>
          </>
        }
      >
        <SprintStageEditor variant="board" stages={editStages} workflows={workflows} onChange={setEditStages} />
      </Modal>

      <Modal
        open={pmImportOpen}
        onClose={() => setPmImportOpen(false)}
        title="从 PM 工具导入"
        description="拉取迭代中的 Issue 并创建为工作项；已关联的 Issue 会自动跳过。"
        footer={
          <>
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setPmImportOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy || !pmIterationId.trim() || (pmProvider === "tapd" && !pmWorkspaceId.trim())}
              onClick={() => void onImportPm()}
            >
              {busy ? "导入中…" : "导入"}
            </button>
          </>
        }
      >
        <div className="stack-sm">
          <label className="stack-xs">
            <span className="text-sm">PM 工具</span>
            <select className="input" value={pmProvider} onChange={(e) => setPmProvider(e.target.value as PmProvider)}>
              {(Object.keys(PM_PROVIDER_LABEL) as PmProvider[]).map((key) => (
                <option key={key} value={key}>
                  {PM_PROVIDER_LABEL[key]}
                </option>
              ))}
            </select>
          </label>
          {pmProvider === "tapd" ? (
            <label className="stack-xs">
              <span className="text-sm">TAPD 项目 ID</span>
              <input
                className="input"
                value={pmWorkspaceId}
                onChange={(e) => setPmWorkspaceId(e.target.value)}
                placeholder="例如 12345678"
              />
            </label>
          ) : null}
          <label className="stack-xs">
            <span className="text-sm">{pmProvider === "jira" ? "Jira Sprint ID" : "迭代 ID"}</span>
            <input
              className="input"
              value={pmIterationId}
              onChange={(e) => setPmIterationId(e.target.value)}
              placeholder={pmProvider === "jira" ? "例如 42" : "TAPD iteration_id"}
            />
          </label>
          <label className="stack-xs">
            <span className="text-sm">同步模式</span>
            <select className="input" value={pmSyncMode} onChange={(e) => setPmSyncMode(e.target.value as "link_only" | "bidirectional")}>
              <option value="link_only">仅关联（不回写 PM）</option>
              <option value="bidirectional">双向同步（工作流成功后更新 PM 状态）</option>
            </select>
          </label>
        </div>
      </Modal>

      {pmLinkTarget ? (
        <WorkItemPmLinkModal
          open
          onClose={() => setPmLinkTarget(null)}
          workspaceId={workspaceId}
          productId={productId}
          workItemId={pmLinkTarget.id}
          workItemTitle={pmLinkTarget.title}
          external={pmLinkTarget.external}
          readOnly={board?.sprint.status === "archived" || board?.sprint.status === "completed"}
          onChanged={() => {
            void loadBoard();
            if (selectedWorkItem?.id === pmLinkTarget.id) {
              void product.getWorkItem(workspaceId, productId, pmLinkTarget.id).then((detail) => {
                setSelectedWorkItem((prev) => (prev?.id === detail.id ? { ...prev, ...detail } : prev));
              }).catch(() => undefined);
            }
          }}
        />
      ) : null}

      <Modal
        open={itemOpen}
        onClose={() => setItemOpen(false)}
        title="添加工作项"
        description={createItemStageName ? `添加到「${createItemStageName}」列` : undefined}
        footer={
          <>
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setItemOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy || !itemTitle.trim() || !itemStageId}
              onClick={() => void onCreateWorkItem()}
            >
              {busy ? "添加中…" : "添加"}
            </button>
          </>
        }
      >
        <label className="stack-xs">
          <span className="text-sm">标题</span>
          <input
            className="input"
            value={itemTitle}
            onChange={(e) => setItemTitle(e.target.value)}
            placeholder="工作项标题"
            autoFocus
            onKeyDown={(e) => {
              if (e.key === "Enter" && itemTitle.trim() && itemStageId && !busy) {
                e.preventDefault();
                void onCreateWorkItem();
              }
            }}
          />
        </label>
      </Modal>

      {executeContext ? (
        <SprintWorkItemExecuteModal
          open={executeModalOpen}
          onClose={() => {
            setExecuteModalOpen(false);
            setExecuteContext(null);
          }}
          workspaceId={workspaceId}
          productId={productId}
          stage={executeContext.stage}
          workflowId={executeContext.workflowId}
          workItems={boardWorkItems}
          initialSelectedId={executeContext.item.id}
          onExecuted={() => {
            void loadBoard();
            void loadSprints();
          }}
        />
      ) : null}

      <WorkItemDetailDrawer
        open={workItemDrawerOpen}
        onClose={() => {
          setWorkItemDrawerOpen(false);
          setSelectedWorkItem(null);
        }}
        workspaceId={workspaceId}
        productId={productId}
        item={selectedWorkItem}
        stages={boardStages}
        workflows={workflows}
        sprintStatus={board?.sprint.status ?? "planning"}
        onChanged={() => {
          void loadBoard();
          void loadSprints();
        }}
        onDeleted={() => {
          void loadBoard();
          void loadSprints();
        }}
        onRequestExecute={({ stageId, workflowId }) => {
          if (!selectedWorkItem) return;
          const stage = boardStages.find((row) => row.id === stageId);
          if (stage) openExecuteWorkflow(selectedWorkItem, stage, workflowId);
        }}
      />
    </>
  );
}
