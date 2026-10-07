"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { EnvironmentVM, ProductVM, ProductWorkflowVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import { InlineAlert } from "@/components/ui/InlineAlert";
import {
  buildApprovalConfig,
  buildHookSettingsConfig,
  emptyReleasePlanFormValues,
  type ReleasePlanFormValues,
  type ReleasePlanSprintOption,
} from "./ReleasePlanForm";
import { ReleasePlanFormFields } from "./ReleasePlanFormFields";
import { ReleasePlanJobEditor } from "./ReleasePlanJobEditor";
import { ReleasePlanStatusSteps } from "./ReleasePlanStatusSteps";
import { buildWorkflowJobParameters, mergeWorkflowJobParameters } from "./release-plan-workflow-params";
import { fromDatetimeLocalValue } from "./release-plan-utils";

interface ReleasePlanCreatePageProps {
  workspaceId: string;
  defaultProductId?: string;
  defaultSprintId?: string;
}

function buildCreatePayload(form: ReleasePlanFormValues) {
  return {
    name: form.name.trim(),
    description: form.description.trim() || undefined,
    release_window_start: fromDatetimeLocalValue(form.windowStart),
    release_window_end: fromDatetimeLocalValue(form.windowEnd),
    approval_enabled: form.approvalEnabled,
    approval_config: buildApprovalConfig(form),
    schedule_execute_time: form.scheduleExecuteTime ? fromDatetimeLocalValue(form.scheduleExecuteTime) : undefined,
    manager_id: form.managerId || undefined,
    sprint_id: form.sprintId || undefined,
    hook_settings: buildHookSettingsConfig(form),
  };
}

function buildUpdatePayload(form: ReleasePlanFormValues) {
  const sprintId = form.sprintId.trim();
  return {
    name: form.name.trim(),
    description: form.description.trim() || undefined,
    release_window_start: form.windowStart,
    release_window_end: form.windowEnd,
    approval_enabled: form.approvalEnabled,
    approval_config: buildApprovalConfig(form),
    hook_settings: buildHookSettingsConfig(form),
    schedule_execute_time: form.scheduleExecuteTime || undefined,
    clear_schedule_time: !form.scheduleExecuteTime,
    manager_id: form.managerId || undefined,
    ...(sprintId ? { sprint_id: sprintId } : { clear_sprint_id: true }),
  };
}

export function ReleasePlanCreatePage({
  workspaceId,
  defaultProductId,
  defaultSprintId,
}: ReleasePlanCreatePageProps) {
  const router = useRouter();
  const addMenuRef = useRef<HTMLDivElement>(null);
  const [products, setProducts] = useState<ProductVM[]>([]);
  const [sprintOptions, setSprintOptions] = useState<ReleasePlanSprintOption[]>([]);
  const [extraSprintOptions, setExtraSprintOptions] = useState<ReleasePlanSprintOption[]>([]);
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [productId, setProductId] = useState("");
  const [planId, setPlanId] = useState<string | null>(null);
  const [plan, setPlan] = useState<ReleasePlanVM | null>(null);
  const [form, setForm] = useState<ReleasePlanFormValues>(() => {
    const empty = emptyReleasePlanFormValues();
    return { ...empty, sprintId: defaultSprintId ?? "" };
  });
  const [managerName, setManagerName] = useState("");
  const [busy, setBusy] = useState(false);
  const [jobsBusy, setJobsBusy] = useState(false);
  const [addMenuOpen, setAddMenuOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mergedSprintOptions = useMemo(() => {
    const map = new Map<string, ReleasePlanSprintOption>();
    for (const item of [...extraSprintOptions, ...sprintOptions]) {
      map.set(item.id, item);
    }
    return [...map.values()];
  }, [extraSprintOptions, sprintOptions]);

  const canAddContent = Boolean(productId && form.name.trim());
  const contentBusy = busy || jobsBusy;

  useEffect(() => {
    void product
      .listProducts(workspaceId)
      .then((items) => {
        setProducts(items);
        const picked =
          defaultProductId && items.some((p) => p.id === defaultProductId)
            ? defaultProductId
            : items[0]?.id ?? "";
        setProductId(picked);
      })
      .catch(() => setProducts([]));
  }, [workspaceId, defaultProductId]);

  useEffect(() => {
    if (!productId) {
      setSprintOptions([]);
      setWorkflows([]);
      setEnvironments([]);
      return;
    }
    if (!defaultSprintId) {
      setForm((prev) => ({ ...prev, sprintId: "" }));
    }
    void product
      .listSprints(workspaceId, productId)
      .then((items) =>
        setSprintOptions(
          items.map((sprint) => ({
            id: sprint.id,
            name: sprint.name,
            external: sprint.external,
          })),
        ),
      )
      .catch(() => setSprintOptions([]));

    void Promise.all([
      product.listProductWorkflows(workspaceId, productId),
      product.listEnvironments(workspaceId, productId),
    ])
      .then(([wfItems, envItems]) => {
        setWorkflows(wfItems);
        setEnvironments(envItems);
      })
      .catch(() => {
        setWorkflows([]);
        setEnvironments([]);
      });
  }, [workspaceId, productId, defaultSprintId]);

  useEffect(() => {
    if (defaultSprintId) {
      setForm((prev) => ({ ...prev, sprintId: defaultSprintId }));
    }
  }, [defaultSprintId]);

  useEffect(() => {
    if (!addMenuOpen) return;
    const onPointerDown = (event: MouseEvent) => {
      if (!addMenuRef.current?.contains(event.target as Node)) {
        setAddMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [addMenuOpen]);

  const refreshPlan = useCallback(
    async (id: string) => {
      const detail = await product.getReleasePlan(workspaceId, productId, id);
      setPlan(detail);
      return detail;
    },
    [workspaceId, productId],
  );

  async function resolveWorkflow(workflowId: string): Promise<ProductWorkflowVM | null> {
    const cached = workflows.find((item) => item.id === workflowId);
    if (cached?.stages?.length) return cached;
    try {
      return await product.getProductWorkflow(workspaceId, productId, workflowId);
    } catch {
      return cached ?? null;
    }
  }

  function handleSprintOptionsChange(options: ReleasePlanSprintOption[]) {
    setExtraSprintOptions((prev) => {
      const map = new Map<string, ReleasePlanSprintOption>();
      for (const item of [...prev, ...options]) {
        map.set(item.id, item);
      }
      return [...map.values()];
    });
  }

  async function ensurePlanCreated(): Promise<ReleasePlanVM | null> {
    if (!productId) {
      setError("请选择目标产品");
      return null;
    }
    if (!form.name.trim()) {
      setError("请填写计划名称");
      return null;
    }
    if (plan && planId) return plan;

    setJobsBusy(true);
    setError(null);
    try {
      const created = await product.createReleasePlan(workspaceId, productId, buildCreatePayload(form));
      const detail = await refreshPlan(created.id);
      setPlanId(created.id);
      return detail;
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建发布计划失败");
      return null;
    } finally {
      setJobsBusy(false);
    }
  }

  async function onAddTextJob() {
    const current = await ensurePlanCreated();
    if (!current) return;
    setAddMenuOpen(false);
    setJobsBusy(true);
    setError(null);
    try {
      await product.addReleasePlanJob(workspaceId, productId, current.id, {
        name: `检查项 ${(current.jobs?.length ?? 0) + 1}`,
        type: "text",
        text_content: "<p>发布前检查清单</p>",
        text_format: "html",
        sort_order: current.jobs?.length ?? 0,
      });
      await refreshPlan(current.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加发布项失败");
    } finally {
      setJobsBusy(false);
    }
  }

  async function onAddWorkflowJob(
    workflowId: string,
    options?: { environmentId?: string; branch?: string },
  ) {
    const current = await ensurePlanCreated();
    if (!current || !workflowId) return;
    setAddMenuOpen(false);
    setJobsBusy(true);
    setError(null);
    try {
      const wf = (await resolveWorkflow(workflowId)) ?? workflows.find((w) => w.id === workflowId);
      const parameters =
        options?.environmentId || options?.branch
          ? buildWorkflowJobParameters(wf, options.environmentId, options.branch)
          : undefined;
      await product.addReleasePlanJob(workspaceId, productId, current.id, {
        name: wf?.name ?? "工作流发布",
        type: "workflow",
        sort_order: current.jobs?.length ?? 0,
        workflow_ref: {
          mode: "reference",
          workflow_id: workflowId,
          ...(parameters && Object.keys(parameters).length > 0 ? { parameters } : {}),
        },
      });
      await refreshPlan(current.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加工作流发布项失败");
    } finally {
      setJobsBusy(false);
    }
  }

  async function onMoveJob(jobId: string, direction: "up" | "down") {
    if (!plan?.jobs || !planId) return;
    const sorted = [...plan.jobs].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
    const idx = sorted.findIndex((j) => j.id === jobId);
    if (idx < 0) return;
    const swap = direction === "up" ? idx - 1 : idx + 1;
    if (swap < 0 || swap >= sorted.length) return;
    const ids = sorted.map((j) => j.id);
    [ids[idx], ids[swap]] = [ids[swap], ids[idx]];
    setJobsBusy(true);
    try {
      await product.reorderReleasePlanJobs(workspaceId, productId, planId, ids);
      await refreshPlan(planId);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "排序失败");
    } finally {
      setJobsBusy(false);
    }
  }

  async function onReorderJobs(jobIds: string[]) {
    if (!planId) return;
    setJobsBusy(true);
    try {
      await product.reorderReleasePlanJobs(workspaceId, productId, planId, jobIds);
      await refreshPlan(planId);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "排序失败");
    } finally {
      setJobsBusy(false);
    }
  }

  async function onWorkflowParamsChange(
    jobId: string,
    workflowId: string,
    patch: { environmentId?: string; branch?: string | null },
  ) {
    if (!planId || !plan) return;
    setJobsBusy(true);
    try {
      const wf = await resolveWorkflow(workflowId);
      const job = plan.jobs?.find((item) => item.id === jobId);
      const parameters = mergeWorkflowJobParameters(job?.workflow_ref?.parameters, wf, patch);
      await product.updateReleasePlanJob(workspaceId, productId, planId, jobId, {
        workflow_ref: {
          mode: "reference",
          workflow_id: workflowId,
          parameters,
        },
      });
      await refreshPlan(planId);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新工作流参数失败");
    } finally {
      setJobsBusy(false);
    }
  }

  async function onSaveDraft() {
    if (!productId) {
      setError("请选择目标产品");
      return;
    }
    if (!form.name.trim()) {
      setError("请填写计划名称");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      let savedId = planId;
      if (planId) {
        await product.updateReleasePlan(workspaceId, productId, planId, buildUpdatePayload(form));
      } else {
        savedId = (await product.createReleasePlan(workspaceId, productId, buildCreatePayload(form))).id;
      }
      router.push(`/products/${productId}?tab=releases&planId=${savedId}`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  const releaseContentAction = (
    <div className="release-plan-create-add-menu" ref={addMenuRef}>
      <button
        type="button"
        className="btn-secondary text-sm"
        disabled={contentBusy || !canAddContent}
        title={!canAddContent ? "请先选择产品并填写计划名称" : undefined}
        aria-expanded={addMenuOpen}
        onClick={() => setAddMenuOpen((open) => !open)}
      >
        + 添加
      </button>
      {addMenuOpen ? (
        <div className="release-plan-create-add-dropdown" role="menu">
          <button type="button" className="release-plan-create-add-option" role="menuitem" onClick={() => void onAddTextJob()}>
            检查项
          </button>
          <button
            type="button"
            className="release-plan-create-add-option"
            role="menuitem"
            disabled={workflows.length === 0}
            title={workflows.length === 0 ? "请先在「工作流」Tab 创建工作流" : undefined}
            onClick={() => void onAddWorkflowJob(workflows[0]?.id ?? "")}
          >
            工作流
          </button>
        </div>
      ) : null}
    </div>
  );

  const releaseContent =
    plan && planId ? (
      <ReleasePlanJobEditor
        workspaceId={workspaceId}
        productId={productId}
        plan={plan}
        workflows={workflows}
        environments={environments}
        canEdit
        busy={jobsBusy}
        onMove={onMoveJob}
        onReorder={onReorderJobs}
        onDelete={async (jobId) => {
          setJobsBusy(true);
          try {
            await product.deleteReleasePlanJob(workspaceId, productId, planId, jobId);
            await refreshPlan(planId);
          } catch (err) {
            setError(err instanceof BffError ? err.message : "删除失败");
          } finally {
            setJobsBusy(false);
          }
        }}
        onUpdate={async (jobId, patch) => {
          const isTextOnly =
            patch.text_content !== undefined &&
            Object.keys(patch).every((key) => key === "text_content" || key === "text_format");
          if (!isTextOnly) setJobsBusy(true);
          try {
            await product.updateReleasePlanJob(workspaceId, productId, planId, jobId, patch);
            if (!isTextOnly) await refreshPlan(planId);
          } catch (err) {
            setError(err instanceof BffError ? err.message : "更新失败");
          } finally {
            if (!isTextOnly) setJobsBusy(false);
          }
        }}
        onAddText={onAddTextJob}
        onAddWorkflow={onAddWorkflowJob}
        onWorkflowParamsChange={onWorkflowParamsChange}
      />
    ) : null;

  return (
    <div className="release-plan-create-page">
      <header className="release-plan-create-toolbar">
        <div className="release-plan-create-toolbar-inner">
          <Breadcrumb
            items={[
              { label: "发布计划", href: "/publish/release-plans" },
              { label: "新建" },
            ]}
          />
          <div className="release-plan-create-toolbar-row">
            <h1 className="release-plan-create-title">新建发布计划</h1>
            <div className="release-plan-create-toolbar-actions">
              <Link href="/publish/release-plans" className="btn-secondary text-sm">
                取消
              </Link>
              <button
                type="button"
                className="btn btn-primary text-sm"
                disabled={busy || !productId || !form.name.trim()}
                onClick={() => void onSaveDraft()}
              >
                {busy ? "保存中…" : "保存草稿"}
              </button>
            </div>
          </div>
        </div>
      </header>

      <div className="release-plan-create-workspace">
        <div className="release-plan-create-sheet">
          <div className="release-plan-create-sheet-main">
            {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
            <ReleasePlanFormFields
              workspaceId={workspaceId}
              productId={productId}
              products={products}
              onProductChange={setProductId}
              values={form}
              variant="page"
              sprintOptions={mergedSprintOptions}
              onSprintOptionsChange={handleSprintOptionsChange}
              onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
              managerName={managerName}
              onManagerNameChange={setManagerName}
              releaseContent={releaseContent}
              releaseContentAction={releaseContentAction}
            />
          </div>

          <aside className="release-plan-create-rail" aria-label="发布进度">
            <span className="release-plan-create-rail-badge">规划中</span>
            <p className="release-plan-create-rail-label">发布进度</p>
            <ReleasePlanStatusSteps status="planning" approvalEnabled={form.approvalEnabled} layout="vertical" />
            <div className="release-plan-create-rail-mobile-actions">
              <button
                type="button"
                className="btn btn-primary text-sm w-full"
                disabled={busy || !productId || !form.name.trim()}
                onClick={() => void onSaveDraft()}
              >
                {busy ? "保存中…" : "保存草稿"}
              </button>
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}
