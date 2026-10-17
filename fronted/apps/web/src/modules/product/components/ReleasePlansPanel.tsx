"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { identity, product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { EnvironmentVM, ProductWorkflowVM, ReleasePlanJobVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail, type ProductMasterListItem } from "./ProductMasterDetail";
import { useWorkspaceMembers } from "../hooks/useWorkspaceMembers";
import { ReleasePlanAuditList } from "./ReleasePlanAuditList";
import { ReleasePlanVersionHistory } from "./ReleasePlanVersionHistory";
import {
  ReleasePlanForm,
  buildApprovalConfig,
  buildHookSettingsConfig,
  type ReleasePlanFormValues,
  type ReleasePlanSprintOption,
} from "./ReleasePlanForm";
import { ReleasePlanCollabBanner } from "./ReleasePlanCollabBanner";
import { ReleasePlanExternalCheckBanner } from "./ReleasePlanExternalCheckBanner";
import { useReleasePlanCollab } from "../hooks/useReleasePlanCollab";
import { ReleasePlanNamePromptModal } from "./ReleasePlanNamePromptModal";
import { ReleasePlanJobEditor } from "./ReleasePlanJobEditor";
import { ReleasePlanStatusSteps } from "./ReleasePlanStatusSteps";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import { ActionMenu } from "./ActionMenu";
import { productDetailTabIcons } from "./product-detail-icons";
import {
  buildWorkflowJobParameters,
  mergeWorkflowJobParameters,
  type WorkflowJobOptions,
  type WorkflowJobParamsPatch,
} from "./release-plan-workflow-params";
import { formatMissingInputs, planMissingInputs } from "./release-plan-runtime-inputs";
import { ReleasePlanParametersSection } from "./ReleasePlanParametersSection";
import {
  formatReleaseWindow,
  releasePlanCanDelete,
  releasePlanStatusBadgeClass,
  releasePlanStatusLabel,
} from "./release-plan-utils";

interface ReleasePlansPanelProps {
  workspaceId: string;
  productId: string;
  sprintId?: string;
  embedded?: boolean;
  initialPlanId?: string | null;
}

const PAGE_SIZE = 50;

const STEP_STATUSES = new Set(["planning", "wait_for_approval", "executing", "success"]);

const CANCELLABLE_STATUSES = new Set([
  "planning",
  "wait_for_approval",
  "executing",
  "wait_for_finish_planning_external_check",
  "wait_for_execute_external_check",
  "wait_for_all_done_external_check",
]);

function plainText(html?: string | null) {
  return (html ?? "")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function buildCreateHref(productId: string, sprintId?: string) {
  const params = new URLSearchParams({ productId });
  if (sprintId) params.set("sprintId", sprintId);
  return `/publish/release-plans/new?${params.toString()}`;
}

export function ReleasePlansPanel({ workspaceId, productId, sprintId, embedded, initialPlanId }: ReleasePlansPanelProps) {
  const router = useRouter();
  const { memberName } = useWorkspaceMembers(workspaceId);
  const [items, setItems] = useState<ReleasePlanVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(initialPlanId ?? null);
  const [selected, setSelected] = useState<ReleasePlanVM | null>(null);
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [sprintOptions, setSprintOptions] = useState<ReleasePlanSprintOption[]>([]);
  const [extraSprintOptions, setExtraSprintOptions] = useState<ReleasePlanSprintOption[]>([]);
  const [detailTab, setDetailTab] = useState<"jobs" | "audit" | "versions">("jobs");
  const [copyOpen, setCopyOpen] = useState(false);
  const [saveTemplateOpen, setSaveTemplateOpen] = useState(false);
  const [templateSaved, setTemplateSaved] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [cancelOpen, setCancelOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [currentMemberId, setCurrentMemberId] = useState<string | undefined>();
  const itemsCountRef = useRef(0);
  const scopeRef = useRef("");
  const appliedInitialRef = useRef<string | null>(null);
  const [jobDoneTarget, setJobDoneTarget] = useState<string | null>(null);
  const [jobDoneRemark, setJobDoneRemark] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [missingFocus, setMissingFocus] = useState<string[]>([]);

  const itemsRef = useRef<ReleasePlanVM[]>([]);

  useEffect(() => {
    itemsRef.current = items;
    itemsCountRef.current = items.length;
  }, [items]);

  useEffect(() => {
    void identity
      .getSession()
      .then((me) => setCurrentMemberId(me?.id))
      .catch(() => setCurrentMemberId(undefined));
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(searchQuery.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [searchQuery]);

  /** silent：操作后静默刷新，保留已加载的条数；search：搜索词变化，不闪烁加载态 */
  const load = useCallback(async (options?: { silent?: boolean; search?: boolean }) => {
    const scope = `${productId}:${sprintId ?? ""}`;
    const sameScope = scopeRef.current === scope;
    scopeRef.current = scope;
    const showSpinner = !options?.silent && !(options?.search && sameScope);
    if (showSpinner) {
      setLoading(true);
      setError(null);
    }
    try {
      const limit = options?.silent && sameScope ? Math.max(PAGE_SIZE, itemsCountRef.current) : PAGE_SIZE;
      const page = await product.listReleasePlansPage(workspaceId, productId, {
        sprintId,
        q: debouncedQuery || undefined,
        limit,
      });
      setItems(page.items);
      setHasMore(page.has_more);
      const applyInitial = Boolean(initialPlanId) && appliedInitialRef.current !== initialPlanId;
      if (applyInitial) appliedInitialRef.current = initialPlanId ?? null;
      setSelectedId((prev) => {
        if (applyInitial) return initialPlanId ?? null;
        if (sameScope && prev) return prev;
        return page.items[0]?.id ?? null;
      });
    } catch (err) {
      if (!options?.silent) {
        setError(err instanceof BffError ? err.message : "加载发布计划失败");
      }
    } finally {
      if (showSpinner) setLoading(false);
    }
  }, [workspaceId, productId, sprintId, initialPlanId, debouncedQuery]);

  useEffect(() => {
    void load({ search: true });
  }, [load]);

  async function loadMore() {
    setLoadingMore(true);
    try {
      const page = await product.listReleasePlansPage(workspaceId, productId, {
        sprintId,
        q: debouncedQuery || undefined,
        limit: PAGE_SIZE,
        offset: items.length,
      });
      setItems((prev) => {
        const seen = new Set(prev.map((item) => item.id));
        return [...prev, ...page.items.filter((item) => !seen.has(item.id))];
      });
      setHasMore(page.has_more);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载更多失败");
    } finally {
      setLoadingMore(false);
    }
  }

  const loadDetail = useCallback(
    async (planId: string) => {
      try {
        const detail = await product.getReleasePlan(workspaceId, productId, planId);
        setSelected(detail);
        setItems((prev) =>
          prev.map((item) =>
            item.id === planId ? { ...item, name: detail.name, status: detail.status } : item,
          ),
        );
      } catch (err) {
        setSelected(null);
        // 链接中的计划可能已被删除：静默回退到列表第一项
        if (err instanceof BffError && (err.status === 404 || err.code === "RELEASE_PLAN_NOT_FOUND")) {
          const rest = itemsRef.current.filter((item) => item.id !== planId);
          setItems(rest);
          setSelectedId(rest[0]?.id ?? null);
          return;
        }
        setError(err instanceof BffError ? err.message : "加载详情失败");
      }
    },
    [workspaceId, productId],
  );

  useEffect(() => {
    if (!selectedId) {
      setSelected(null);
      return;
    }
    void loadDetail(selectedId);
  }, [selectedId, loadDetail]);

  const loadWorkflows = useCallback(async () => {
    try {
      const [wfItems, envItems, sprints] = await Promise.all([
        product.listProductWorkflows(workspaceId, productId),
        product.listEnvironments(workspaceId, productId),
        product.listSprints(workspaceId, productId),
      ]);
      setWorkflows(wfItems);
      setEnvironments(envItems);
      setSprintOptions(
        sprints.map((sprint) => ({
          id: sprint.id,
          name: sprint.name,
          external: sprint.external,
        })),
      );
    } catch {
      setWorkflows([]);
      setEnvironments([]);
      setSprintOptions([]);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void loadWorkflows();
  }, [loadWorkflows]);

  async function resolveWorkflow(workflowId: string): Promise<ProductWorkflowVM | null> {
    const cached = workflows.find((item) => item.id === workflowId);
    if (cached?.stages?.length) return cached;
    try {
      return await product.getProductWorkflow(workspaceId, productId, workflowId);
    } catch {
      return cached ?? null;
    }
  }

  const needsPolling =
    selected?.status === "executing" &&
    selected.jobs?.some((job) => job.status === "running" || job.type === "workflow");

  useEffect(() => {
    if (!selected || !needsPolling) return;
    const timer = window.setInterval(() => {
      void loadDetail(selected.id);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [selected, needsPolling, loadDetail]);

  async function refreshDetail(options?: { reloadList?: boolean }) {
    if (!selectedId) return;
    await loadDetail(selectedId);
    if (options?.reloadList) await load({ silent: true });
  }

  function openCreatePage() {
    router.push(buildCreateHref(productId, embedded ? sprintId : undefined));
  }

  async function onUpdate(values: ReleasePlanFormValues) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.updateReleasePlan(workspaceId, productId, selected.id, {
        name: values.name,
        description: values.description || undefined,
        release_window_start: values.windowStart,
        release_window_end: values.windowEnd,
        approval_enabled: values.approvalEnabled,
        approval_config: buildApprovalConfig(values),
        hook_settings: buildHookSettingsConfig(values),
        schedule_execute_time: values.scheduleExecuteTime || undefined,
        clear_schedule_time: !values.scheduleExecuteTime,
        manager_id: values.managerId || undefined,
        ...resolveSprintUpdatePayload(values.sprintId),
      });
      setEditOpen(false);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新失败");
    } finally {
      setBusy(false);
    }
  }

  async function onAddTextJob() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.addReleasePlanJob(workspaceId, productId, selected.id, {
        name: `检查项 ${(selected.jobs?.length ?? 0) + 1}`,
        type: "text",
        text_content: "<p>发布前检查清单</p>",
        text_format: "html",
        sort_order: selected.jobs?.length ?? 0,
      });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加发布项失败");
    } finally {
      setBusy(false);
    }
  }

  async function onAddWorkflowJob(
    workflowId: string,
    options?: WorkflowJobOptions,
  ) {
    if (!selected || !workflowId) return;
    setBusy(true);
    try {
      const wf = (await resolveWorkflow(workflowId)) ?? workflows.find((w) => w.id === workflowId);
      const parameters = options
        ? buildWorkflowJobParameters(
            wf,
            options.environmentId,
            options.branch,
            options.projectIds,
            options.refOverrides,
          )
        : undefined;
      await product.addReleasePlanJob(workspaceId, productId, selected.id, {
        name: wf?.name ?? "工作流发布",
        type: "workflow",
        sort_order: selected.jobs?.length ?? 0,
        workflow_ref: {
          mode: "reference",
          workflow_id: workflowId,
          ...(parameters && Object.keys(parameters).length > 0 ? { parameters } : {}),
        },
      });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加工作流发布项失败");
    } finally {
      setBusy(false);
    }
  }

  async function onWorkflowParamsChange(
    jobId: string,
    workflowId: string,
    patch: WorkflowJobParamsPatch,
  ) {
    if (!selected) return;
    setBusy(true);
    try {
      const wf = await resolveWorkflow(workflowId);
      const job = selected.jobs?.find((item) => item.id === jobId);
      const parameters = mergeWorkflowJobParameters(job?.workflow_ref?.parameters, wf, patch);
      await product.updateReleasePlanJob(workspaceId, productId, selected.id, jobId, {
        workflow_ref: {
          mode: job?.workflow_ref?.mode ?? "reference",
          workflow_id: workflowId,
          parameters,
        },
      });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新工作流参数失败");
    } finally {
      setBusy(false);
    }
  }

  function resolveSprintPayload(id?: string) {
    const value = id?.trim();
    return value ? { sprint_id: value } : {};
  }

  function resolveSprintUpdatePayload(id: string) {
    const value = id.trim();
    if (embedded && sprintId) {
      return { sprint_id: sprintId };
    }
    return value ? { sprint_id: value } : { clear_sprint_id: true };
  }

  async function onReorderJobs(jobIds: string[]) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.reorderReleasePlanJobs(workspaceId, productId, selected.id, jobIds);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "排序失败");
    } finally {
      setBusy(false);
    }
  }

  async function onMoveJob(jobId: string, direction: "up" | "down") {
    if (!selected?.jobs) return;
    const sorted = [...selected.jobs].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
    const idx = sorted.findIndex((j) => j.id === jobId);
    if (idx < 0) return;
    const swap = direction === "up" ? idx - 1 : idx + 1;
    if (swap < 0 || swap >= sorted.length) return;
    const ids = sorted.map((j) => j.id);
    [ids[idx], ids[swap]] = [ids[swap], ids[idx]];
    setBusy(true);
    try {
      await product.reorderReleasePlanJobs(workspaceId, productId, selected.id, ids);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "排序失败");
    } finally {
      setBusy(false);
    }
  }

  async function onFinishPlanning() {
    if (!selected) return;
    const missing = planMissingInputs(selected);
    if (missing.length > 0) {
      setError(formatMissingInputs(missing));
      setDetailTab("jobs");
      setMissingFocus(missing.map((item) => item.jobId));
      return;
    }
    setBusy(true);
    try {
      await product.finishReleasePlanPlanning(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "完成规划失败");
    } finally {
      setBusy(false);
    }
  }

  async function onPlanParametersChange(parameters: Record<string, string>) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.updateReleasePlan(workspaceId, productId, selected.id, { parameters });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存发布参数失败");
    } finally {
      setBusy(false);
    }
  }

  async function onApprove() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.approveReleasePlan(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "审批失败");
    } finally {
      setBusy(false);
    }
  }

  async function onReject() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.rejectReleasePlan(workspaceId, productId, selected.id, { reason: rejectReason.trim() || undefined });
      setRejectOpen(false);
      setRejectReason("");
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "驳回失败");
    } finally {
      setBusy(false);
    }
  }

  async function onReopen() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.reopenReleasePlanPlanning(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "重新编辑失败");
    } finally {
      setBusy(false);
    }
  }

  async function onCancel() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.cancelReleasePlan(workspaceId, productId, selected.id);
      setCancelOpen(false);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "取消失败");
    } finally {
      setBusy(false);
    }
  }

  async function onDelete() {
    if (!deleteTarget) return;
    const targetId = deleteTarget.id;
    setBusy(true);
    try {
      await product.deleteReleasePlan(workspaceId, productId, targetId);
      setDeleteTarget(null);
      setItems((prev) => prev.filter((item) => item.id !== targetId));
      if (selectedId === targetId) {
        setSelectedId(null);
        setSelected(null);
      }
      await load({ silent: true });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setBusy(false);
    }
  }

  async function onExecute() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.executeReleasePlan(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "执行失败");
    } finally {
      setBusy(false);
    }
  }

  async function onJobDone(jobId: string, remark?: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.markReleasePlanJobDone(workspaceId, productId, selected.id, jobId, { remark: remark?.trim() || undefined });
      setJobDoneTarget(null);
      setJobDoneRemark("");
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "操作失败");
    } finally {
      setBusy(false);
    }
  }

  async function onJobSkip(jobId: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.skipReleasePlanJob(workspaceId, productId, selected.id, jobId);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "跳过失败");
    } finally {
      setBusy(false);
    }
  }

  async function onJobRetry(jobId: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.retryReleasePlanJob(workspaceId, productId, selected.id, jobId);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "重试失败");
    } finally {
      setBusy(false);
    }
  }

  async function onGenerateFromWorkItems(workItemIds: string[]) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.addReleasePlanJobsFromWorkItems(workspaceId, productId, selected.id, workItemIds);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "生成检查项失败");
    } finally {
      setBusy(false);
    }
  }

  const executionStep = (() => {
    if (selected?.status !== "executing") return null;
    const scheduledAt = selected.schedule_execute_time ? new Date(selected.schedule_execute_time) : null;
    if (scheduledAt && scheduledAt.getTime() > Date.now()) {
      return {
        label: "等待定时执行",
        hint: `将于 ${scheduledAt.toLocaleString("zh-CN", { hour12: false })} 自动开始`,
        onRun: null,
      };
    }
    const current = [...(selected.jobs ?? [])]
      .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
      .find((job) => job.status !== "done" && job.status !== "skipped");
    if (!current) return { label: "收尾中", hint: "所有发布项已完成，等待收尾回调", onRun: null };
    if (current.type === "text") {
      return {
        label: "确认完成",
        hint: `当前步骤「${current.name}」需要人工确认`,
        onRun: () => setJobDoneTarget(current.id),
      };
    }
    if (current.status === "running") {
      return { label: "执行中…", hint: `「${current.name}」正在运行，完成后自动进入下一步`, onRun: null };
    }
    if (current.status === "failed") {
      return { label: "重试", hint: `「${current.name}」执行失败`, onRun: () => void onJobRetry(current.id) };
    }
    return { label: "开始执行", hint: `下一步：运行工作流「${current.name}」`, onRun: () => void onExecute() };
  })();

  function renderJobActions(job: ReleasePlanJobVM) {
    if (selected?.status !== "executing") return null;
    if (job.type === "text" && job.status !== "done" && job.status !== "skipped") {
      return (
        <div className="flex gap-1">
          <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => setJobDoneTarget(job.id)}>
            完成
          </button>
          <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onJobSkip(job.id)}>
            跳过
          </button>
        </div>
      );
    }
    if (job.type === "workflow" && job.status === "failed") {
      return (
        <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onJobRetry(job.id)}>
          重试
        </button>
      );
    }
    if (job.type === "workflow" && job.status !== "done" && job.status !== "skipped") {
      return (
        <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onJobSkip(job.id)}>
          跳过
        </button>
      );
    }
    return null;
  }

  const listItems = useMemo(
    () =>
      items.map((row) => ({
        id: row.id,
        title: row.name,
        meta: releasePlanStatusLabel(row.status),
        metaClassName: `badge ${releasePlanStatusBadgeClass(row.status)} product-master-item-badge`,
      })),
    [items],
  );

  const statusById = useMemo(() => new Map(items.map((row) => [row.id, row.status])), [items]);

  function listItemActions(item: ProductMasterListItem) {
    const status = statusById.get(item.id) ?? "";
    const deletable = releasePlanCanDelete(status);
    return [
      {
        label: "删除",
        danger: true,
        disabled: !deletable,
        title: deletable ? undefined : "进行中的计划需先取消后才能删除",
        onSelect: () => setDeleteTarget({ id: item.id, name: item.title }),
      },
    ];
  }

  const canEditJobs = selected?.status === "planning";
  const descriptionText = useMemo(() => plainText(selected?.description), [selected?.description]);
  const usesExternalApproval =
    selected?.approval_config?.provider === "feishu" || selected?.approval_config?.provider === "dingtalk";
  const approverLabels = (selected?.approval_config?.approver_ids ?? [])
    .map((id) => memberName(id))
    .filter(Boolean);

  const collab = useReleasePlanCollab({
    enabled: selected?.status === "planning",
    workspaceId,
    productId,
    planId: selected?.id,
    memberId: currentMemberId,
    section: detailTab === "jobs" ? "jobs" : "plan",
    location:
      typeof window !== "undefined"
        ? window.location
        : { protocol: "http:", hostname: "localhost", host: "localhost", origin: "http://localhost" },
    isDev: process.env.NODE_ENV === "development",
  });

  async function onCopyPlan(name: string) {
    if (!selected) return;
    setBusy(true);
    try {
      const copied = await product.copyReleasePlan(workspaceId, productId, selected.id, { name });
      setCopyOpen(false);
      await load();
      setSelectedId(copied.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "复制失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSaveTemplate(name: string, description?: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.saveReleasePlanTemplate(workspaceId, productId, selected.id, { name, description });
      setSaveTemplateOpen(false);
      setTemplateSaved(true);
      window.setTimeout(() => setTemplateSaved(false), 3000);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存模板失败");
    } finally {
      setBusy(false);
    }
  }

  const sprintFormOptions = useMemo(() => {
    const map = new Map<string, ReleasePlanSprintOption>();
    for (const item of [...extraSprintOptions, ...sprintOptions]) {
      map.set(item.id, item);
    }
    const options = [...map.values()];
    if (selected?.sprint_id && !map.has(selected.sprint_id)) {
      options.unshift({
        id: selected.sprint_id,
        name: selected.sprint?.name ?? selected.sprint_id,
        external: selected.sprint?.external_ref as ReleasePlanSprintOption["external"],
      });
    }
    if (embedded && sprintId && !options.some((item) => item.id === sprintId)) {
      options.unshift({ id: sprintId, name: sprintId });
    }
    return options;
  }, [sprintOptions, extraSprintOptions, selected?.sprint_id, selected?.sprint?.name, selected?.sprint?.external_ref, embedded, sprintId]);

  function handleSprintOptionsChange(options: ReleasePlanSprintOption[]) {
    setExtraSprintOptions((prev) => {
      const map = new Map<string, ReleasePlanSprintOption>();
      for (const item of [...prev, ...options]) {
        map.set(item.id, item);
      }
      return [...map.values()];
    });
  }

  return (
    <>
      {error ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle={embedded ? "本迭代发布" : "发布计划"}
        searchPlaceholder="搜索计划"
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        serverSearch
        items={listItems}
        itemActions={listItemActions}
        listEmpty={
          debouncedQuery ? <div className="product-master-list-status">没有匹配「{debouncedQuery}」的计划</div> : undefined
        }
        listFooter={
          hasMore ? (
            <button
              type="button"
              className="product-master-load-more"
              disabled={loadingMore}
              onClick={() => void loadMore()}
            >
              {loadingMore ? "加载中…" : "加载更多"}
            </button>
          ) : null
        }
        selectedId={selectedId}
        onSelect={setSelectedId}
        onAdd={openCreatePage}
        addAriaLabel="新建发布计划"
        loading={loading}
        showMain={Boolean(selected)}
        emptyMain={
          <ProductDetailEmptyState
            title={embedded ? "本迭代暂无发布计划" : "暂无发布计划，请点击 + 新建"}
            description={
              embedded
                ? "为本迭代创建发布计划，规划发布项并执行上线。"
                : "创建迭代计划后，可规划发布项、提交审批并执行发布。"
            }
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={openCreatePage}>
                新建发布计划
              </button>
            }
          />
        }
      >
        {selected ? (
          <div className="product-master-main-inner release-plan-detail stack-md">
            <header className="release-plan-detail-header">
              <div className="release-plan-detail-head">
                <div className="release-plan-detail-heading">
                  <div className="release-plan-detail-title-row">
                    <h2 className="release-plan-detail-title">{selected.name}</h2>
                    {!STEP_STATUSES.has(selected.status) ? (
                      <span className={`badge shrink-0 ${releasePlanStatusBadgeClass(selected.status)}`}>
                        {releasePlanStatusLabel(selected.status)}
                      </span>
                    ) : null}
                  </div>
                  <p className="release-plan-detail-meta">
                    <span>{formatReleaseWindow(selected.release_window_start, selected.release_window_end)}</span>
                    {selected.manager_id ? <span>负责人 {memberName(selected.manager_id)}</span> : null}
                    {selected.sprint?.name ? <span>迭代 {selected.sprint.name}</span> : null}
                    {selected.schedule_execute_time ? (
                      <span>定时 {new Date(selected.schedule_execute_time).toLocaleString("zh-CN")}</span>
                    ) : null}
                    {selected.approval_enabled && !usesExternalApproval && approverLabels.length > 0 ? (
                      <span>审批人 {approverLabels.join("、")}</span>
                    ) : null}
                    {usesExternalApproval ? (
                      <span>
                        {selected.approval_config?.provider === "feishu" ? "飞书审批" : "钉钉审批"}
                        {selected.approval_config?.instance?.status ? ` · ${selected.approval_config.instance.status}` : ""}
                      </span>
                    ) : null}
                  </p>
                </div>

                <div className="release-plan-detail-actions">
                  {templateSaved ? <span className="release-plan-detail-flash">模板已保存</span> : null}
                  {selected.status === "planning" ? (
                    <>
                      <button type="button" className="btn btn-secondary text-sm" disabled={busy} onClick={() => setEditOpen(true)}>
                        编辑
                      </button>
                      <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onFinishPlanning()}>
                        完成规划
                      </button>
                    </>
                  ) : null}
                  {selected.status === "wait_for_approval" && !usesExternalApproval ? (
                    <>
                      <button type="button" className="btn btn-secondary text-sm" disabled={busy} onClick={() => setRejectOpen(true)}>
                        驳回
                      </button>
                      <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onApprove()}>
                        审批通过
                      </button>
                    </>
                  ) : null}
                  {selected.status === "wait_for_approval" && usesExternalApproval ? (
                    <span className="release-plan-detail-flash">等待飞书/钉钉审批回调</span>
                  ) : null}
                  {executionStep ? (
                    <>
                      <span className="release-plan-detail-flash">{executionStep.hint}</span>
                      <button
                        type="button"
                        className="btn btn-primary text-sm"
                        disabled={busy || !executionStep.onRun}
                        onClick={() => executionStep.onRun?.()}
                      >
                        {executionStep.label}
                      </button>
                    </>
                  ) : null}
                  {selected.status === "denied" ? (
                    <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onReopen()}>
                      重新编辑
                    </button>
                  ) : null}
                  <ActionMenu
                    ariaLabel="更多操作"
                    disabled={busy}
                    triggerClassName="action-menu-trigger is-boxed"
                    trigger={productDetailTabIcons.more}
                    items={[
                      { label: "复制计划", onSelect: () => setCopyOpen(true) },
                      { label: "存为模板", onSelect: () => setSaveTemplateOpen(true) },
                      CANCELLABLE_STATUSES.has(selected.status)
                        ? { label: "取消计划", danger: true, separatorBefore: true, onSelect: () => setCancelOpen(true) }
                        : null,
                      releasePlanCanDelete(selected.status)
                        ? {
                            label: "删除计划",
                            danger: true,
                            separatorBefore: !CANCELLABLE_STATUSES.has(selected.status),
                            onSelect: () => setDeleteTarget({ id: selected.id, name: selected.name }),
                          }
                        : null,
                    ]}
                  />
                </div>
              </div>

              {descriptionText && descriptionText !== selected.name ? (
                <p className="release-plan-detail-desc">{descriptionText}</p>
              ) : null}

              <ReleasePlanStatusSteps status={selected.status} approvalEnabled={selected.approval_enabled} />
            </header>

            <ReleasePlanCollabBanner workspaceId={workspaceId} editors={collab.editors} />
            <ReleasePlanExternalCheckBanner plan={selected} />

            <section className="release-plan-detail-body">
              <div className="release-plan-tabs" role="tablist" aria-label="详情视图">
                {(
                  [
                    { key: "jobs", label: "发布项", count: selected.jobs?.length ?? 0 },
                    { key: "audit", label: "操作记录" },
                    { key: "versions", label: "版本历史", count: selected.version ? `v${selected.version}` : undefined },
                  ] as const
                ).map((tab) => (
                  <button
                    key={tab.key}
                    type="button"
                    role="tab"
                    aria-selected={detailTab === tab.key}
                    className={`release-plan-tab${detailTab === tab.key ? " is-active" : ""}`}
                    onClick={() => setDetailTab(tab.key)}
                  >
                    {tab.label}
                    {"count" in tab && tab.count ? <span className="release-plan-tab-count">{tab.count}</span> : null}
                  </button>
                ))}
              </div>

            {detailTab === "jobs" ? (
              <>
              <ReleasePlanParametersSection
                plan={selected}
                editable={selected.status === "planning"}
                busy={busy}
                onChange={onPlanParametersChange}
              />
              <ReleasePlanJobEditor
                workspaceId={workspaceId}
                productId={productId}
                plan={selected}
                workflows={workflows}
                environments={environments}
                canEdit={canEditJobs}
                busy={busy}
                onMove={onMoveJob}
                onReorder={onReorderJobs}
                onDelete={async (jobId) => {
                  setBusy(true);
                  try {
                    await product.deleteReleasePlanJob(workspaceId, productId, selected.id, jobId);
                    await refreshDetail();
                  } catch (err) {
                    setError(err instanceof BffError ? err.message : "删除失败");
                  } finally {
                    setBusy(false);
                  }
                }}
                onUpdate={async (jobId, patch) => {
                  const isTextOnly =
                    patch.text_content !== undefined &&
                    Object.keys(patch).every((key) => key === "text_content" || key === "text_format");
                  if (!isTextOnly) setBusy(true);
                  try {
                    await product.updateReleasePlanJob(workspaceId, productId, selected.id, jobId, patch);
                    if (isTextOnly) {
                      setSelected((prev) =>
                        prev
                          ? {
                              ...prev,
                              jobs: (prev.jobs ?? []).map((job) =>
                                job.id === jobId
                                  ? {
                                      ...job,
                                      ...(patch.text_content !== undefined
                                        ? { text_content: patch.text_content }
                                        : {}),
                                      ...(patch.text_format !== undefined
                                        ? { text_format: patch.text_format }
                                        : {}),
                                    }
                                  : job,
                              ),
                            }
                          : prev,
                      );
                    } else {
                      await refreshDetail();
                    }
                  } catch (err) {
                    setError(err instanceof BffError ? err.message : "更新失败");
                  } finally {
                    if (!isTextOnly) setBusy(false);
                  }
                }}
                onAddText={onAddTextJob}
                onAddWorkflow={onAddWorkflowJob}
                onWorkflowParamsChange={onWorkflowParamsChange}
                renderJobActions={renderJobActions}
                memberName={memberName}
                onImportWorkItems={onGenerateFromWorkItems}
                focusJobIds={missingFocus}
              />
              </>
            ) : detailTab === "audit" ? (
              <ReleasePlanAuditList
                workspaceId={workspaceId}
                productId={productId}
                planId={selected.id}
                memberName={memberName}
              />
            ) : (
              <ReleasePlanVersionHistory
                workspaceId={workspaceId}
                productId={productId}
                planId={selected.id}
                currentVersion={selected.version}
                refreshKey={selected.version}
                active={detailTab === "versions"}
                memberName={memberName}
              />
            )}
            </section>
          </div>
        ) : null}
      </ProductMasterDetail>

      <ReleasePlanForm
        open={editOpen}
        title="编辑发布计划"
        workspaceId={workspaceId}
        productId={productId}
        busy={busy}
        initial={selected}
        sprintOptions={sprintFormOptions}
        sprintLocked={Boolean(embedded && sprintId)}
        onSprintOptionsChange={handleSprintOptionsChange}
        onClose={() => setEditOpen(false)}
        onSubmit={onUpdate}
      />

      <ReleasePlanNamePromptModal
        open={copyOpen}
        title="复制发布计划"
        description="为新计划指定名称，发布项与配置将一并复制。"
        initialName={selected ? `${selected.name} 副本` : ""}
        namePlaceholder="例如 v2.4 副本"
        submitLabel="确认复制"
        submittingLabel="复制中…"
        busy={busy}
        onClose={() => setCopyOpen(false)}
        onSubmit={({ name }) => onCopyPlan(name)}
      />

      <ReleasePlanNamePromptModal
        open={saveTemplateOpen}
        title="存为模板"
        description="保存当前计划的发布项结构，便于后续快速创建。"
        initialName={selected ? `${selected.name} 模板` : ""}
        initialDescription={selected?.description ?? ""}
        showDescription
        nameLabel="模板名称"
        descriptionLabel="模板描述"
        submitLabel="保存模板"
        submittingLabel="保存中…"
        busy={busy}
        onClose={() => setSaveTemplateOpen(false)}
        onSubmit={({ name, description }) => onSaveTemplate(name, description)}
      />

      <Modal
        open={rejectOpen}
        onClose={() => setRejectOpen(false)}
        title="驳回发布计划"
        footer={
          <ReleasePlanModalFooter
            onCancel={() => setRejectOpen(false)}
            submitLabel="确认驳回"
            submittingLabel="提交中…"
            busy={busy}
            onSubmit={() => void onReject()}
          />
        }
      >
        <label className="stack-xs text-sm">
          <span className="text-muted-foreground">驳回原因（可选）</span>
          <textarea className="input min-h-[80px]" value={rejectReason} onChange={(e) => setRejectReason(e.target.value)} />
        </label>
      </Modal>

      <Modal
        open={cancelOpen}
        onClose={() => setCancelOpen(false)}
        title="取消发布计划"
        description="取消后计划将无法继续执行，此操作不可撤销。"
        footer={
          <ReleasePlanModalFooter
            onCancel={() => setCancelOpen(false)}
            submitLabel="确认取消"
            submittingLabel="取消中…"
            busy={busy}
            onSubmit={() => void onCancel()}
          />
        }
      >
        <p className="text-sm text-muted-foreground">确定要取消「{selected?.name}」吗？</p>
      </Modal>

      <Modal
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="删除发布计划"
        description="删除后计划及其发布项、操作记录将被永久移除，此操作不可撤销。"
        footer={
          <ReleasePlanModalFooter
            onCancel={() => setDeleteTarget(null)}
            submitLabel="确认删除"
            submittingLabel="删除中…"
            busy={busy}
            onSubmit={() => void onDelete()}
          />
        }
      >
        <p className="text-sm text-muted-foreground">确定要删除「{deleteTarget?.name}」吗？</p>
      </Modal>

      <Modal
        open={Boolean(jobDoneTarget)}
        onClose={() => {
          setJobDoneTarget(null);
          setJobDoneRemark("");
        }}
        title="标记检查项完成"
        footer={
          <ReleasePlanModalFooter
            onCancel={() => {
              setJobDoneTarget(null);
              setJobDoneRemark("");
            }}
            submitLabel="确认完成"
            submittingLabel="提交中…"
            busy={busy}
            onSubmit={() => jobDoneTarget && void onJobDone(jobDoneTarget, jobDoneRemark)}
          />
        }
      >
        <label className="stack-xs text-sm">
          <span className="text-muted-foreground">完成备注（可选）</span>
          <textarea className="input min-h-[80px]" value={jobDoneRemark} onChange={(e) => setJobDoneRemark(e.target.value)} placeholder="记录检查结果或注意事项" />
        </label>
      </Modal>
    </>
  );
}
