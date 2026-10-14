"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  ProductWorkflowVM,
  SprintStage,
  SprintStatus,
  WorkItemCardVM,
  WorkItemExecutionVM,
  WorkItemPriority,
  WorkItemType,
  WorkItemVM,
} from "@sparkops/contracts/product";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { WorkItemAssigneeField } from "./WorkItemAssigneeField";
import { WorkItemAttributeFields } from "./WorkItemAttributeFields";
import { moveWorkItemErrorMessage } from "./WorkItemCardMenu";
import { WorkItemPmChip } from "./WorkItemPmChip";
import { WorkItemPmLinkModal } from "./WorkItemPmLinkModal";

const EXEC_STATUS_LABEL: Record<string, string> = {
  running: "运行中",
  success: "成功",
  failed: "失败",
  cancelled: "已取消",
};

interface WorkItemDetailDrawerProps {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
  productId: string;
  item: WorkItemCardVM | null;
  stages: SprintStage[];
  workflows: ProductWorkflowVM[];
  sprintStatus: SprintStatus;
  onChanged: () => void;
  onDeleted: (itemId: string) => void;
  onRequestExecute?: (params: { stageId: string; workflowId: string }) => void;
}

function formatDuration(startedAt: string, finishedAt?: string) {
  const start = Date.parse(startedAt);
  const end = finishedAt ? Date.parse(finishedAt) : Date.now();
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return "-";
  const seconds = Math.floor((end - start) / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}m`;
}

function formatDateTime(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", { hour12: false });
}

export function WorkItemDetailDrawer({
  open,
  onClose,
  workspaceId,
  productId,
  item,
  stages,
  workflows,
  sprintStatus,
  onChanged,
  onDeleted,
  onRequestExecute,
}: WorkItemDetailDrawerProps) {
  const [detail, setDetail] = useState<WorkItemVM | null>(null);
  const [executions, setExecutions] = useState<WorkItemExecutionVM[]>([]);
  const [selectedStageId, setSelectedStageId] = useState("");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pmModalOpen, setPmModalOpen] = useState(false);
  const loadSeqRef = useRef(0);
  const itemId = item?.id;

  const workflowNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const wf of workflows) map.set(wf.id, wf.name);
    return map;
  }, [workflows]);

  const load = useCallback(async () => {
    if (!itemId) return;
    const seq = ++loadSeqRef.current;
    setLoading(true);
    setError(null);
    try {
      const [detailData, execItems] = await Promise.all([
        product.getWorkItem(workspaceId, productId, itemId),
        product.listWorkItemExecutions(workspaceId, productId, itemId),
      ]);
      if (seq !== loadSeqRef.current) return;
      setDetail(detailData);
      setDescription(detailData.description ?? "");
      setExecutions(execItems);
      setSelectedStageId(detailData.stage_id);
    } catch (err) {
      if (seq !== loadSeqRef.current) return;
      setError(err instanceof BffError ? err.message : "加载工作项失败");
      setDetail(null);
      setExecutions([]);
    } finally {
      if (seq === loadSeqRef.current) setLoading(false);
    }
  }, [itemId, workspaceId, productId]);

  useEffect(() => {
    if (!open || !itemId) {
      loadSeqRef.current += 1;
      setDetail(null);
      setExecutions([]);
      setError(null);
      setLoading(false);
      setPmModalOpen(false);
      return;
    }
    if (item?.stage_id) setSelectedStageId(item.stage_id);
    void load();
  }, [open, itemId, item?.stage_id, load]);

  const selectedStage = useMemo(
    () => stages.find((stage) => stage.id === selectedStageId) ?? null,
    [stages, selectedStageId],
  );

  const stageExecutions = useMemo(
    () => executions.filter((exec) => exec.stage_id === selectedStageId),
    [executions, selectedStageId],
  );

  const activityItems = useMemo(() => {
    const rows: Array<{ at: string; text: string }> = [];
    if (detail?.created_at) {
      rows.push({ at: detail.created_at, text: "创建了工作项" });
    }
    for (const exec of executions) {
      const wfName = exec.workflow_name ?? workflowNameById.get(exec.workflow_id) ?? "工作流";
      const label = EXEC_STATUS_LABEL[exec.status] ?? exec.status;
      rows.push({
        at: exec.finished_at ?? exec.started_at,
        text: `执行 ${wfName} · ${label}`,
      });
    }
    return rows.sort((a, b) => Date.parse(b.at) - Date.parse(a.at));
  }, [detail?.created_at, executions, workflowNameById]);

  async function saveDescription() {
    if (!item || !detail) return;
    const next = description.trim();
    if ((detail.description ?? "") === next) return;
    setBusy(true);
    setError(null);
    try {
      await product.updateWorkItem(workspaceId, productId, item.id, { description: next });
      setDetail({ ...detail, description: next });
      onChanged();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存描述失败");
    } finally {
      setBusy(false);
    }
  }

  async function saveAttributes(patch: { type?: WorkItemType; priority?: WorkItemPriority }) {
    if (!item || !detail) return;
    setBusy(true);
    setError(null);
    try {
      await product.updateWorkItem(workspaceId, productId, item.id, patch);
      setDetail({ ...detail, ...patch });
      onChanged();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function saveAssignee(nextId: string | null, nextName: string) {
    if (!item || !detail) return;
    const currentId = detail.assignee_id ?? "";
    const targetId = nextId ?? "";
    if (currentId === targetId) return;
    setBusy(true);
    setError(null);
    try {
      if (!nextId) {
        await product.updateWorkItem(workspaceId, productId, item.id, { clear_assignee: true });
        setDetail({ ...detail, assignee_id: undefined, assignee_name: undefined });
      } else {
        await product.updateWorkItem(workspaceId, productId, item.id, { assignee_id: nextId });
        setDetail({ ...detail, assignee_id: nextId, assignee_name: nextName });
      }
      onChanged();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更改负责人失败");
    } finally {
      setBusy(false);
    }
  }

  async function onMove(targetStageId: string) {
    if (!itemId || !view || targetStageId === view.stage_id) return;
    setBusy(true);
    setError(null);
    try {
      await product.moveWorkItem(workspaceId, productId, itemId, {
        stage_id: targetStageId,
        sort_order: 0,
        version: view.version,
      });
      await load();
      onChanged();
    } catch (err) {
      setError(moveWorkItemErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onDelete() {
    if (!item) return;
    setBusy(true);
    setError(null);
    try {
      await product.deleteWorkItem(workspaceId, productId, item.id);
      onDeleted(item.id);
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除工作项失败");
    } finally {
      setBusy(false);
    }
  }

  const view = detail ?? item;

  const currentStage = useMemo(
    () => (view ? stages.find((stage) => stage.id === view.stage_id) ?? null : null),
    [view, stages],
  );

  const readOnly = sprintStatus === "archived" || sprintStatus === "completed";
  const stageBindings = selectedStage?.workflows ?? [];
  const canExecute = sprintStatus === "active" && stageBindings.length > 0;
  const isRunning = view?.delivery?.last_workflow_status === "running";

  return (
    <>
    <Drawer
      open={open}
      onClose={onClose}
      title={view?.title ?? "工作项"}
      panelClassName="drawer-panel-wide work-item-detail-drawer"
      footer={
        sprintStatus !== "archived" && sprintStatus !== "completed" ? (
          <div className="work-item-detail-footer">
            <label className="work-item-detail-move">
              <span className="sr-only">移动到</span>
              <select
                className="input text-sm"
                value={view?.stage_id ?? ""}
                disabled={busy || !view || sprintStatus !== "active"}
                onChange={(e) => void onMove(e.target.value)}
              >
                {stages.map((stage) => (
                  <option key={stage.id} value={stage.id}>
                    移动到 · {stage.name}
                  </option>
                ))}
              </select>
            </label>
            <button type="button" className="btn-ghost text-sm text-red-600" disabled={busy} onClick={() => void onDelete()}>
              删除
            </button>
          </div>
        ) : null
      }
    >
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {view ? (
        <div className="work-item-detail stack-md">
          {loading ? <p className="text-xs text-muted-foreground">刷新数据中…</p> : null}
          <div className="work-item-detail-meta">
            {currentStage ? (
              <span className="work-item-detail-stage-badge">{currentStage.name}</span>
            ) : null}
          </div>

          <WorkItemAttributeFields
            type={view.type}
            priority={view.priority}
            disabled={busy || readOnly}
            onTypeChange={(value) => void saveAttributes({ type: value })}
            onPriorityChange={(value) => void saveAttributes({ priority: value })}
          />

          <div className="work-item-detail-fields grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="stack-xs">
              <span className="text-sm font-medium">负责人</span>
              <WorkItemAssigneeField
                workspaceId={workspaceId}
                assigneeId={view.assignee_id}
                assigneeName={view.assignee_name}
                disabled={busy || sprintStatus === "archived" || sprintStatus === "completed"}
                onSelect={(memberId, label) => saveAssignee(memberId, label)}
              />
            </div>
            <div className="stack-xs">
              <span className="text-sm font-medium">PM 关联</span>
              <WorkItemPmChip
                external={view.external}
                disabled={busy}
                onManage={() => setPmModalOpen(true)}
                onLink={
                  sprintStatus !== "archived" && sprintStatus !== "completed"
                    ? () => setPmModalOpen(true)
                    : undefined
                }
              />
            </div>
            <label className="stack-xs md:col-span-2">
              <span className="text-sm font-medium">描述</span>
              <textarea
                className="input min-h-24"
                value={description}
                disabled={busy || sprintStatus === "archived" || sprintStatus === "completed"}
                onChange={(e) => setDescription(e.target.value)}
                onBlur={() => void saveDescription()}
                placeholder="添加工作项描述…"
              />
            </label>
          </div>

          <section className="work-item-detail-section stack-sm">
            <h3 className="work-item-detail-section-title">执行记录</h3>
            <div className="work-item-detail-exec-layout">
              <nav className="work-item-detail-stage-nav" aria-label="阶段">
                {stages.map((stage) => (
                  <button
                    key={stage.id}
                    type="button"
                    className={`work-item-detail-stage-nav-item${stage.id === selectedStageId ? " is-active" : ""}`}
                    onClick={() => setSelectedStageId(stage.id)}
                  >
                    {stage.name}
                  </button>
                ))}
              </nav>

              <div className="work-item-detail-exec-main stack-sm">
                {stageBindings.length === 0 ? (
                  <p className="text-sm text-muted-foreground">该阶段未绑定工作流</p>
                ) : (
                  stageBindings.map((binding) => {
                    const wfName =
                      binding.workflow_name ?? workflowNameById.get(binding.workflow_id) ?? binding.workflow_id;
                    return (
                      <div key={binding.workflow_id} className="work-item-detail-workflow-row">
                        <span className="work-item-detail-workflow-tag">{wfName}</span>
                        {canExecute && onRequestExecute ? (
                          <button
                            type="button"
                            className="work-item-detail-play-btn"
                            disabled={busy || isRunning}
                            title="配置并执行工作流"
                            aria-label={`配置并执行 ${wfName}`}
                            onClick={() => {
                              onRequestExecute({
                                stageId: selectedStageId,
                                workflowId: binding.workflow_id,
                              });
                            }}
                          >
                            ▶
                          </button>
                        ) : null}
                      </div>
                    );
                  })
                )}

                <div className="work-item-detail-table-wrap">
                  <table className="work-item-detail-table">
                    <thead>
                      <tr>
                        <th>ID</th>
                        <th>状态</th>
                        <th>持续时间</th>
                        <th>代码信息</th>
                        <th>交付物</th>
                        <th>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {stageExecutions.length === 0 ? (
                        <tr>
                          <td colSpan={6} className="work-item-detail-table-empty">
                            暂无数据
                          </td>
                        </tr>
                      ) : (
                        stageExecutions.map((exec) => {
                          const branch = exec.artifacts?.branch ?? exec.artifacts?.commit_sha ?? "-";
                          const deliverables = [
                            exec.artifacts?.images?.map((img) => img.tag).join(", "),
                            exec.artifacts?.deploy_envs?.join(", "),
                          ]
                            .filter(Boolean)
                            .join(" · ");
                          return (
                            <tr key={exec.id}>
                              <td className="font-mono text-xs" title={exec.id}>
                                {exec.id.slice(0, 8)}
                              </td>
                              <td>{EXEC_STATUS_LABEL[exec.status] ?? exec.status}</td>
                              <td>{formatDuration(exec.started_at, exec.finished_at)}</td>
                              <td className="text-xs">{branch || "-"}</td>
                              <td className="text-xs">{deliverables || "-"}</td>
                              <td>
                                <Link
                                  className="text-xs text-primary hover:underline"
                                  href={`/products/${productId}/workflows/${exec.workflow_id}/runs/${exec.workflow_run_id}`}
                                >
                                  查看
                                </Link>
                              </td>
                            </tr>
                          );
                        })
                      )}
                    </tbody>
                  </table>
                </div>
                <p className="text-xs text-muted-foreground text-right">
                  {wfNameHint(stageExecutions, workflowNameById)}
                </p>
              </div>
            </div>
          </section>

          <section className="work-item-detail-section stack-sm">
            <h3 className="work-item-detail-section-title">活动记录</h3>
            <ul className="work-item-detail-activity">
              {activityItems.length === 0 ? (
                <li className="text-sm text-muted-foreground">暂无活动</li>
              ) : (
                activityItems.map((row, index) => (
                  <li key={`${row.at}-${index}`} className="work-item-detail-activity-item">
                    <time className="work-item-detail-activity-time">{formatDateTime(row.at)}</time>
                    <span>{row.text}</span>
                  </li>
                ))
              )}
            </ul>
          </section>
        </div>
      ) : null}
    </Drawer>
    {item && pmModalOpen ? (
      <WorkItemPmLinkModal
        open={pmModalOpen}
        onClose={() => setPmModalOpen(false)}
        workspaceId={workspaceId}
        productId={productId}
        workItemId={item.id}
        workItemTitle={view?.title}
        external={view?.external}
        readOnly={sprintStatus === "archived" || sprintStatus === "completed"}
        onChanged={() => {
          void load();
          onChanged();
        }}
      />
    ) : null}
    </>
  );
}

function wfNameHint(
  executions: WorkItemExecutionVM[],
  workflowNameById: Map<string, string>,
): string {
  if (executions.length === 0) return "共 0 条执行记录";
  return `共 ${executions.length} 条执行记录`;
}
