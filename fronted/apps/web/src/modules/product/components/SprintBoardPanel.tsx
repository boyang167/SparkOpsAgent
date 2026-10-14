"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  StageWorkflowBinding,
  WorkItemCardVM,
  WorkItemPriority,
  WorkItemType,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { useSprintBoardSync } from "@/modules/product/hooks/useSprintBoardSync";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail, type ProductMasterItemAction, type ProductMasterListItem } from "./ProductMasterDetail";
import { ReleasePlansPanel } from "./ReleasePlansPanel";
import { SprintStageEditor } from "./SprintStageEditor";
import { buildStageMigrations } from "./sprint-stage-migrations";
import { isValidStageKey, normalizeStageForSave, withSortOrder } from "./sprint-process-templates";
import { SprintWorkItemExecuteModal } from "./SprintWorkItemExecuteModal";
import { PM_PROVIDER_LABEL } from "./pm-labels";
import { WorkItemAttributeFields } from "./WorkItemAttributeFields";
import {
  DEFAULT_WORK_ITEM_PRIORITY,
  DEFAULT_WORK_ITEM_TYPE,
  WORK_ITEM_PRIORITY_OPTIONS,
  WORK_ITEM_TYPE_OPTIONS,
  workItemMetaParts,
} from "./work-item-attributes";
import { moveWorkItemErrorMessage, WorkItemCardMenu } from "./WorkItemCardMenu";
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

const COLUMN_PAGE_SIZE = 40;

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

type BoardItemSortKey = "manual" | "type" | "priority";

const TYPE_RANK = new Map(WORK_ITEM_TYPE_OPTIONS.map((option, index) => [option.value, index]));
const PRIORITY_RANK = new Map(WORK_ITEM_PRIORITY_OPTIONS.map((option, index) => [option.value, index]));

function compareBoardItems(
  a: WorkItemCardVM,
  b: WorkItemCardVM,
  key: BoardItemSortKey,
  direction: "asc" | "desc",
) {
  if (key !== "manual") {
    const left = key === "type" ? TYPE_RANK.get(a.type) : PRIORITY_RANK.get(a.priority);
    const right = key === "type" ? TYPE_RANK.get(b.type) : PRIORITY_RANK.get(b.priority);
    const delta = (left ?? Number.MAX_SAFE_INTEGER) - (right ?? Number.MAX_SAFE_INTEGER);
    if (delta !== 0) return direction === "asc" ? delta : -delta;
  }
  return a.sort_order - b.sort_order;
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
  const [itemSort, setItemSort] = useState<{ key: BoardItemSortKey; direction: "asc" | "desc" }>({
    key: "manual",
    direction: "asc",
  });
  const [metrics, setMetrics] = useState<SprintMetricsVM | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createGoal, setCreateGoal] = useState("");
  const [createTemplateId, setCreateTemplateId] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);
  const [wsProcessTemplates, setWsProcessTemplates] = useState<WorkspaceSprintProcessTemplateVM[]>([]);
  const [createTemplatesLoading, setCreateTemplatesLoading] = useState(false);
  const [stagesEditOpen, setStagesEditOpen] = useState(false);
  const [editStages, setEditStages] = useState<SprintStage[]>([]);
  const [editFocus, setEditFocus] = useState<{ session: number; stageId?: string }>({ session: 0 });
  const [itemOpen, setItemOpen] = useState(false);
  const [itemTitle, setItemTitle] = useState("");
  const [itemStageId, setItemStageId] = useState("");
  const [itemType, setItemType] = useState<WorkItemType>(DEFAULT_WORK_ITEM_TYPE);
  const [itemPriority, setItemPriority] = useState<WorkItemPriority>(DEFAULT_WORK_ITEM_PRIORITY);
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
  const [dragItemId, setDragItemId] = useState<string | null>(null);
  const [dropStageId, setDropStageId] = useState<string | null>(null);
  const [columnLimit, setColumnLimit] = useState<Record<string, number>>({});
  const [blockingPlans, setBlockingPlans] = useState<Array<{ id: string; name: string; status: string }>>([]);
  const [activeReleasePlanCount, setActiveReleasePlanCount] = useState(0);
  const [linkedReleasePlanCount, setLinkedReleasePlanCount] = useState(0);
  const [showArchived, setShowArchived] = useState(false);
  const [sprintAction, setSprintAction] = useState<{ kind: "delete" | "archive"; id: string; name: string } | null>(null);
  const [sprintActionError, setSprintActionError] = useState<string | null>(null);

  const loadReleasePlansMeta = useCallback(
    async (sprintId: string) => {
      setLinkedReleasePlanCount(0);
      try {
        const page = await product.listReleasePlansPage(workspaceId, productId, { sprintId, limit: 200 });
        const plans = page.items;
        setActiveReleasePlanCount(
          plans.filter((p) => ["planning", "wait_for_approval", "executing"].includes(p.status)).length,
        );
        setLinkedReleasePlanCount(plans.length);
        setBlockingPlans(
          plans
            .filter((p) => ["executing", "wait_for_approval"].includes(p.status))
            .map((p) => ({ id: p.id, name: p.name, status: p.status })),
        );
      } catch {
        setActiveReleasePlanCount(0);
        setLinkedReleasePlanCount(0);
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
      setSelectedId((prev) => {
        if (prev && items.some((row) => row.id === prev)) return prev;
        return (items.find((row) => row.status !== "archived") ?? items[0])?.id ?? null;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载迭代列表失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  const shownSprintRef = useRef<string | null>(null);
  const boardRequestRef = useRef(0);

  const loadBoard = useCallback(async () => {
    const requestId = ++boardRequestRef.current;
    if (!selectedId) {
      shownSprintRef.current = null;
      setBoard(null);
      return;
    }
    const switching = shownSprintRef.current !== selectedId;
    if (switching) {
      setBoardLoading(true);
      setError(null);
    }
    try {
      const next = await product.getSprintBoard(workspaceId, productId, selectedId);
      if (boardRequestRef.current !== requestId) return;
      shownSprintRef.current = selectedId;
      setBoard(next);
    } catch (err) {
      if (boardRequestRef.current !== requestId) return;
      setError(err instanceof BffError ? err.message : "加载看板失败");
      if (switching) {
        shownSprintRef.current = null;
        setBoard(null);
      }
    } finally {
      if (boardRequestRef.current === requestId && switching) setBoardLoading(false);
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
    setColumnLimit({});
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
    setCreateError(null);
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
      if (document.hidden) return;
      void loadBoard();
    }, 15000);
    return () => window.clearInterval(timer);
  }, [hasRunningItems, selectedId, loadBoard, workItemDrawerOpen, executeModalOpen]);

  const archivedCount = useMemo(() => sprints.filter((s) => s.status === "archived").length, [sprints]);

  const filteredSprints = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return sprints.filter(
      (s) => (showArchived || s.status !== "archived") && (!q || s.name.toLowerCase().includes(q)),
    );
  }, [sprints, searchQuery, showArchived]);

  const sprintStatusById = useMemo(() => new Map(sprints.map((s) => [s.id, s.status])), [sprints]);

  function sprintListActions(item: ProductMasterListItem): ProductMasterItemAction[] {
    const status = sprintStatusById.get(item.id);
    const target = { id: item.id, name: item.title };
    const blockedByPlans = item.id === selectedId && linkedReleasePlanCount > 0;
    const remove: ProductMasterItemAction = {
      label: "删除",
      danger: true,
      disabled: blockedByPlans,
      title: blockedByPlans ? "该迭代下还有关联的发布计划，请先删除这些计划" : undefined,
      onSelect: () => openSprintAction("delete", target),
    };
    if (status === "completed") {
      return [{ label: "归档", onSelect: () => openSprintAction("archive", target) }, remove];
    }
    return [remove];
  }

  function openSprintAction(kind: "delete" | "archive", target: { id: string; name: string }) {
    setSprintActionError(null);
    setSprintAction({ kind, ...target });
  }

  async function onConfirmSprintAction() {
    if (!sprintAction) return;
    const { kind, id } = sprintAction;
    setBusy(true);
    setSprintActionError(null);
    try {
      if (kind === "delete") {
        await product.deleteSprint(workspaceId, productId, id);
      } else {
        await product.archiveSprint(workspaceId, productId, id);
      }
      setSprintAction(null);
      if (selectedId === id && (kind === "delete" || !showArchived)) setSelectedId(null);
      await loadSprints();
    } catch (err) {
      setSprintActionError(
        err instanceof BffError && err.code === "SPRINT_HAS_RELEASE_PLANS"
          ? "该迭代下还有关联的发布计划，请先删除这些计划后再删除迭代"
          : err instanceof BffError
              ? err.message
              : kind === "delete"
                ? "删除迭代失败"
                : "归档迭代失败",
      );
    } finally {
      setBusy(false);
    }
  }

  const listItems = useMemo(
    () =>
      filteredSprints.map((row) => ({
        id: row.id,
        title: row.name,
        meta: STATUS_LABEL[row.status] ?? row.status,
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
    setCreateError(null);
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
      setCreateError(err instanceof BffError ? err.message : "创建迭代失败");
    } finally {
      setBusy(false);
    }
  }

  function openEditStages(focusStageId?: string) {
    if (boardStages.length === 0) return;
    setError(null);
    setEditStages(boardStages.map((stage) => ({ ...stage })));
    setEditFocus((prev) => ({ session: prev.session + 1, stageId: focusStageId }));
    setStagesEditOpen(true);
  }

  async function onSaveStageWorkflows() {
    if (!selectedId) return;
    const stages = boardStages.map((stage) => ({
      ...stage,
      workflows: editStages.find((edited) => edited.id === stage.id)?.workflows ?? [],
    }));
    setBusy(true);
    setError(null);
    try {
      await product.updateSprint(workspaceId, productId, selectedId, { stages });
      setStagesEditOpen(false);
      await loadBoard();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存工作流绑定失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSaveSprintStages() {
    if (!selectedId || !board) return;
    if (board.sprint.status === "active") {
      await onSaveStageWorkflows();
      return;
    }
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
        type: itemType,
        priority: itemPriority,
        project_ids: [],
        labels: [],
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

  function workflowName(binding?: StageWorkflowBinding) {
    if (!binding) return "";
    return binding.workflow_name || workflows.find((wf) => wf.id === binding.workflow_id)?.name || "工作流";
  }

  function stageWorkflowTitle(binding: StageWorkflowBinding) {
    const parts = [binding.trigger_mode === "auto_on_enter" ? "工作项进入此列时自动执行" : "手动执行"];
    if (binding.required_for_exit) parts.push("执行成功后才能移到后面的列");
    return `${workflowName(binding)}：${parts.join("；")}`;
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
    setItemType(DEFAULT_WORK_ITEM_TYPE);
    setItemPriority(DEFAULT_WORK_ITEM_PRIORITY);
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

  async function onDropWorkItem(targetStageId: string, workItemId: string) {
    const item = board?.columns.flatMap((column) => column.work_items).find((row) => row.id === workItemId);
    if (!item || item.stage_id === targetStageId) return;
    const target = board?.columns.find((column) => column.stage.id === targetStageId);
    const sortOrder = (target?.work_items ?? []).reduce((max, row) => Math.max(max, row.sort_order), -1) + 1;
    setBusy(true);
    setError(null);
    try {
      await product.moveWorkItem(workspaceId, productId, item.id, {
        stage_id: targetStageId,
        sort_order: sortOrder,
        version: item.version ?? 0,
      });
      await loadBoard();
      await loadSprints();
    } catch (err) {
      setError(moveWorkItemErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  function renderCard(item: WorkItemCardVM, _columns: SprintBoardColumnVM[]) {
    const runStatus = item.delivery?.last_workflow_status;
    const metaParts = workItemMetaParts(item);
    const canManageCard =
      board?.sprint.status !== "archived" && board?.sprint.status !== "completed";
    const itemStage = boardStages.find((stage) => stage.id === item.stage_id);
    const itemBinding = itemStage?.workflows?.[0];
    const execute =
      board?.sprint.status === "active" && itemStage && itemBinding && runStatus !== "running"
        ? {
            label: `执行「${workflowName(itemBinding)}」`,
            onSelect: () => openExecuteWorkflow(item, itemStage, itemBinding.workflow_id),
          }
        : undefined;
    const canDrag = canManageCard && !busy;
    return (
      <article
        key={item.id}
        className={`sprint-board-card${dragItemId === item.id ? " is-dragging" : ""}`}
        draggable={canDrag}
        onDragStart={(event) => {
          const target = event.target;
          if (target instanceof Element && target.closest(".sprint-board-card-menu-wrap, .work-item-pm-chip")) {
            event.preventDefault();
            return;
          }
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("text/plain", item.id);
          setDragItemId(item.id);
        }}
        onDragEnd={() => {
          setDragItemId(null);
          setDropStageId(null);
        }}
      >
        <div className="sprint-board-card-top">
          <button
            type="button"
            className="sprint-board-card-title sprint-board-card-title-btn"
            onClick={() => openWorkItemDetail(item)}
          >
            {item.title}
          </button>
          {canManageCard ? (
            <WorkItemCardMenu
              item={item}
              stages={boardStages}
              workspaceId={workspaceId}
              productId={productId}
              sprintActive={board?.sprint.status === "planning" || board?.sprint.status === "active"}
              disabled={busy}
              open={cardMenuOpenId === item.id}
              onToggle={() => setCardMenuOpenId((prev) => (prev === item.id ? null : item.id))}
              onClose={() => setCardMenuOpenId(null)}
              onLinkPm={item.external?.issue_key ? undefined : () => setPmLinkTarget(item)}
              execute={execute}
              onChanged={() => {
                void loadBoard();
                void loadSprints();
              }}
            />
          ) : null}
        </div>
        {metaParts.length > 0 || runStatus ? (
          <div className="sprint-board-card-meta">
            {metaParts.map((part) => (
              <span key={part.key} className={`sprint-board-card-tag is-${part.tone}`}>
                {part.label}
              </span>
            ))}
            {runStatus ? (
              <span className={runStatusClass(runStatus)}>{RUN_STATUS_LABEL[runStatus] ?? runStatus}</span>
            ) : null}
          </div>
        ) : null}
        {item.external?.issue_key ? (
          <WorkItemPmChip external={item.external} disabled={busy} compact onManage={() => setPmLinkTarget(item)} />
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
        itemActions={sprintListActions}
        listFooter={
          archivedCount > 0 ? (
            <button type="button" className="product-master-load-more" onClick={() => setShowArchived((prev) => !prev)}>
              {showArchived ? "隐藏已归档" : `显示已归档（${archivedCount}）`}
            </button>
          ) : null
        }
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
            <div className="product-master-main-inner sprint-head">
              <div className="sprint-head-top">
                <div className="sprint-head-main">
                  <div className="sprint-head-title-row">
                    <h2 className="sprint-head-title">{sprint.name}</h2>
                    <span className={`sprint-status is-${sprint.status}`}>
                      <span className="sprint-status-dot" aria-hidden />
                      {STATUS_LABEL[sprint.status] ?? sprint.status}
                    </span>
                    {sprint.work_item_count ? (
                      <span className="sprint-head-progress">
                        已完成 {sprint.done_work_item_count ?? 0} / {sprint.work_item_count}
                      </span>
                    ) : null}
                  </div>
                  {sprint.goal ? <p className="sprint-head-goal">{sprint.goal}</p> : null}
                </div>
                <div className="sprint-head-actions">
                  {sprint.status === "planning" || sprint.status === "active" ? (
                    <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setPmImportOpen(true)}>
                      导入 PM
                    </button>
                  ) : null}
                  {sprint.status === "planning" ? (
                    <>
                      <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => openEditStages()}>
                        编辑看板列
                      </button>
                      <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onStartSprint()}>
                        启动迭代
                      </button>
                    </>
                  ) : null}
                  {sprint.status === "active" ? (
                    <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onCompleteSprint()}>
                      完成迭代
                    </button>
                  ) : null}
                </div>
              </div>

              <div className="release-plan-tabs" role="tablist" aria-label="迭代视图">
                {(
                  [
                    { id: "board", label: "看板" },
                    { id: "release", label: "发布计划", count: activeReleasePlanCount },
                    { id: "metrics", label: "度量" },
                  ] as const
                ).map((tab) => (
                  <button
                    key={tab.id}
                    type="button"
                    role="tab"
                    aria-selected={viewTab === tab.id}
                    className={`release-plan-tab${viewTab === tab.id ? " is-active" : ""}`}
                    onClick={() => setViewTab(tab.id)}
                  >
                    {tab.label}
                    {"count" in tab && tab.count > 0 ? <span className="release-plan-tab-count">{tab.count}</span> : null}
                  </button>
                ))}
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
                    <div className="sprint-metrics">
                      <div className="sprint-metric">
                        <span className="sprint-metric-label">工作项总数</span>
                        <span className="sprint-metric-value">{metrics.total_work_items}</span>
                      </div>
                      <div className="sprint-metric">
                        <span className="sprint-metric-label">已完成</span>
                        <span className="sprint-metric-value is-success">{metrics.done_work_items}</span>
                      </div>
                      <div className="sprint-metric">
                        <span className="sprint-metric-label">阻塞中</span>
                        <span className={`sprint-metric-value${metrics.blocked_work_items > 0 ? " is-warning" : ""}`}>
                          {metrics.blocked_work_items}
                        </span>
                      </div>
                      <div className="sprint-metric">
                        <span className="sprint-metric-label">工作流执行次数</span>
                        <span className="sprint-metric-value">{metrics.deployment_frequency ?? 0}</span>
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
                            <li key={point.date} className="sprint-burndown-item">
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
                {board.columns.some((column) => column.work_items.length > 0) ? (
                  <div className="sprint-board-sort" role="group" aria-label="工作项排序">
                    <span>排序</span>
                    {(
                      [
                        { key: "manual", label: "默认" },
                        { key: "type", label: "类型" },
                        { key: "priority", label: "优先级" },
                      ] as const
                    ).map((option) => {
                      const active = itemSort.key === option.key;
                      return (
                        <button
                          key={option.key}
                          type="button"
                          className={active ? "is-active" : undefined}
                          aria-pressed={active}
                          onClick={() =>
                            setItemSort((prev) =>
                              option.key === "manual"
                                ? { key: "manual", direction: "asc" }
                                : prev.key === option.key
                                  ? { key: option.key, direction: prev.direction === "asc" ? "desc" : "asc" }
                                  : { key: option.key, direction: "asc" },
                            )
                          }
                        >
                          {option.label}
                          {active && option.key !== "manual" ? (itemSort.direction === "asc" ? " ↑" : " ↓") : null}
                        </button>
                      );
                    })}
                  </div>
                ) : null}
                <div className="sprint-board" role="region" aria-label="迭代看板">
                  {board.columns.map((column) => {
                    const items = [...column.work_items].sort((a, b) =>
                      compareBoardItems(a, b, itemSort.key, itemSort.direction),
                    );
                    const visibleCount = columnLimit[column.stage.id] ?? COLUMN_PAGE_SIZE;
                    const visibleItems = items.slice(0, visibleCount);
                    const canDrop = sprint.status === "planning" || sprint.status === "active";
                    return (
                    <section
                      key={column.stage.id}
                      className={`sprint-board-column${dropStageId === column.stage.id ? " is-drop-target" : ""}`}
                      onDragOver={(event) => {
                        if (!canDrop || !dragItemId) return;
                        event.preventDefault();
                        event.dataTransfer.dropEffect = "move";
                        if (dropStageId !== column.stage.id) setDropStageId(column.stage.id);
                      }}
                      onDragLeave={(event) => {
                        const next = event.relatedTarget;
                        if (next instanceof Node && event.currentTarget.contains(next)) return;
                        if (dropStageId === column.stage.id) setDropStageId(null);
                      }}
                      onDrop={(event) => {
                        event.preventDefault();
                        const workItemId = event.dataTransfer.getData("text/plain") || dragItemId;
                        setDragItemId(null);
                        setDropStageId(null);
                        if (workItemId) void onDropWorkItem(column.stage.id, workItemId);
                      }}
                    >
                      <header className="sprint-board-column-head">
                        <div className="sprint-board-column-title-row">
                          <span
                            className="sprint-board-column-dot"
                            style={{ background: column.stage.color || "var(--muted)" }}
                            aria-hidden
                          />
                          <span className="sprint-board-column-title">{column.stage.name}</span>
                          <span className={`sprint-board-column-count${column.wip_limit_exceeded ? " is-over" : ""}`}>
                            {column.wip_count}
                            {column.stage.wip_limit ? ` / ${column.stage.wip_limit}` : ""}
                          </span>
                        </div>
                        {column.wip_limit_exceeded ? <span className="sprint-board-column-warn">超出 WIP 上限</span> : null}
                        {(() => {
                          const binding = column.stage.workflows?.[0];
                          const editable = sprint.status === "planning" || sprint.status === "active";
                          if (!binding && !editable) return null;
                          const content = (
                            <>
                              <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden>
                                <path
                                  d="M9 1.5 3.5 9H8l-1 5.5L12.5 7H8l1-5.5Z"
                                  fill="none"
                                  stroke="currentColor"
                                  strokeWidth="1.3"
                                  strokeLinejoin="round"
                                />
                              </svg>
                              <span className="sprint-board-column-wf-name">{binding ? workflowName(binding) : "绑定工作流"}</span>
                              {binding?.trigger_mode === "auto_on_enter" ? <span className="sprint-board-column-wf-tag">自动</span> : null}
                              {binding?.required_for_exit ? <span className="sprint-board-column-wf-tag">必须成功</span> : null}
                            </>
                          );
                          const className = `sprint-board-column-wf${binding ? " is-bound" : " is-unbound"}`;
                          return editable ? (
                            <button
                              type="button"
                              className={className}
                              disabled={busy}
                              title={binding ? stageWorkflowTitle(binding) : `为「${column.stage.name}」绑定工作流`}
                              onClick={() => openEditStages(column.stage.id)}
                            >
                              {content}
                            </button>
                          ) : (
                            <span className={className} title={binding ? stageWorkflowTitle(binding) : undefined}>
                              {content}
                            </span>
                          );
                        })()}
                      </header>
                      <div className="sprint-board-column-body">
                        {visibleItems.map((item) => renderCard(item, board.columns))}
                        {items.length > visibleItems.length ? (
                          <button
                            type="button"
                            className="sprint-board-column-more"
                            onClick={() =>
                              setColumnLimit((prev) => ({
                                ...prev,
                                [column.stage.id]: (prev[column.stage.id] ?? COLUMN_PAGE_SIZE) + COLUMN_PAGE_SIZE,
                              }))
                            }
                          >
                            还有 {items.length - visibleItems.length} 个
                          </button>
                        ) : null}
                        {sprint.status !== "planning" && sprint.status !== "active" && items.length === 0 ? (
                          <p className="sprint-board-column-empty">暂无工作项</p>
                        ) : null}
                        {sprint.status === "planning" || sprint.status === "active" ? (
                          <button
                            type="button"
                            className="sprint-board-column-add"
                            disabled={busy}
                            onClick={() => openCreateItem(column.stage.id)}
                          >
                            + 添加工作项
                          </button>
                        ) : null}
                      </div>
                    </section>
                    );
                  })}
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
        description="为产品开启一次交付周期。"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy || !createName.trim()} onClick={() => void onCreateSprint()}>
              {busy ? "创建中…" : "创建迭代"}
            </button>
          </div>
        }
      >
        <form
          className="sprint-create"
          onSubmit={(event) => {
            event.preventDefault();
            if (!busy && createName.trim()) void onCreateSprint();
          }}
        >
          {createError ? <InlineAlert tone="error">{createError}</InlineAlert> : null}
          <label className="sprint-create-field">
            <span className="sprint-create-label">
              迭代名称<span className="sprint-create-required" aria-hidden>*</span>
            </span>
            <input
              className="input"
              value={createName}
              autoFocus
              maxLength={128}
              onChange={(e) => setCreateName(e.target.value)}
              placeholder="例如 v3.2.4"
            />
          </label>
          <label className="sprint-create-field">
            <span className="sprint-create-label">
              迭代目标<span className="sprint-create-optional">可选</span>
            </span>
            <textarea
              className="input sprint-create-goal"
              rows={2}
              value={createGoal}
              onChange={(e) => setCreateGoal(e.target.value)}
              placeholder="这次迭代要达成什么"
            />
          </label>
          <div className="sprint-create-field">
            <div className="sprint-create-label-row">
              <span className="sprint-create-label">看板流程</span>
              <Link href="/templates?tab=sprint-process" className="sprint-create-link">
                管理模板
              </Link>
            </div>
            {createTemplatesLoading ? (
              <p className="sprint-create-hint">正在加载模板…</p>
            ) : wsProcessTemplates.length === 0 ? (
              <p className="sprint-create-hint">工作空间暂无流程模板，将使用默认看板列。</p>
            ) : (
              <div className="sprint-create-templates" role="radiogroup" aria-label="看板流程">
                {wsProcessTemplates.map((tpl) => {
                  const selected = tpl.id === createTemplateId;
                  const stages = [...(tpl.stages ?? [])].sort((a, b) => a.sort_order - b.sort_order);
                  return (
                    <button
                      key={tpl.id}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      className={`sprint-create-tpl${selected ? " is-selected" : ""}`}
                      onClick={() => setCreateTemplateId(tpl.id)}
                    >
                      <span className="sprint-create-tpl-radio" aria-hidden />
                      <span className="sprint-create-tpl-body">
                        <span className="sprint-create-tpl-title">
                          {tpl.name}
                          {tpl.template_key === "agile-4" ? <span className="sprint-create-tpl-tag">默认</span> : null}
                        </span>
                        <span className="sprint-create-tpl-stages">
                          {stages.length > 0 ? stages.map((stage) => stage.name).join(" → ") : "未配置看板列"}
                        </span>
                      </span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </form>
      </Modal>

      <Modal
        open={Boolean(sprintAction)}
        onClose={() => !busy && setSprintAction(null)}
        title={sprintAction?.kind === "delete" ? "删除迭代" : "归档迭代"}
        description={
          sprintAction?.kind === "delete"
            ? "迭代及其工作项将被删除，此操作不可撤销。"
            : "归档后迭代不再出现在列表中，可通过「显示已归档」查看，数据与度量完整保留。"
        }
        size="sm"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setSprintAction(null)}>
              取消
            </button>
            <button
              type="button"
              className={`btn-primary text-sm${sprintAction?.kind === "delete" ? " project-service-danger" : ""}`}
              disabled={busy}
              onClick={() => void onConfirmSprintAction()}
            >
              {busy ? "处理中…" : sprintAction?.kind === "delete" ? "确认删除" : "确认归档"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {sprintActionError ? <InlineAlert tone="error">{sprintActionError}</InlineAlert> : null}
          <p className="text-sm text-muted-foreground">
            确定要{sprintAction?.kind === "delete" ? "删除" : "归档"}「{sprintAction?.name}」吗？
          </p>
        </div>
      </Modal>

      <Modal
        open={stagesEditOpen}
        onClose={() => setStagesEditOpen(false)}
        title={sprint?.status === "active" ? "工作流绑定" : "编辑看板列"}
        description={
          sprint?.status === "active"
            ? "为各列配置工作流：手动执行或进入时自动执行，可要求执行成功后才能移到后面的列。"
            : "调整列名称、顺序、颜色与绑定的工作流。删除有卡片的列时，工作项会迁到第一列。"
        }
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
        <SprintStageEditor
          key={editFocus.session}
          variant="board"
          stages={editStages}
          workflows={workflows}
          onChange={setEditStages}
          structureLocked={sprint?.status === "active"}
          initialExpandedIds={editFocus.stageId ? [editFocus.stageId] : undefined}
        />
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
          <div className="modal-footer-actions">
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
          </div>
        }
      >
        <div className="sprint-create">
          <label className="sprint-create-field">
            <span className="sprint-create-label">
              标题<span className="sprint-create-required" aria-hidden>*</span>
            </span>
            <input
              className="input"
              value={itemTitle}
              onChange={(e) => setItemTitle(e.target.value)}
              placeholder="要做的事情"
              autoFocus
              maxLength={512}
              onKeyDown={(e) => {
                if (e.key === "Enter" && itemTitle.trim() && itemStageId && !busy) {
                  e.preventDefault();
                  void onCreateWorkItem();
                }
              }}
            />
          </label>
          <WorkItemAttributeFields
            type={itemType}
            priority={itemPriority}
            disabled={busy}
            onTypeChange={setItemType}
            onPriorityChange={setItemPriority}
          />
        </div>
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
