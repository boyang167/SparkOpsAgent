"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductWorkflowRunSummaryVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { formatDurationMs } from "./workflow-list-utils";
import {
  branchSummary,
  executorLabel,
  formatRunTime,
  projectLabel,
  runStatusLabel,
  runStatusTone,
  serviceGroups,
  type MemberLookup,
} from "./workflow-run-history-utils";

interface WorkflowRunHistoryTableProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
  runs: ProductWorkflowRunSummaryVM[];
  projectNames: Record<string, string>;
  memberNames: Record<string, MemberLookup>;
  onUpdated: () => void;
}

function IconPencil() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden>
      <path d="M4 20h4L18.5 9.5a2.12 2.12 0 0 0-3-3L5 17v3Z" strokeLinejoin="round" />
      <path d="m14 8 2 2" strokeLinecap="round" />
    </svg>
  );
}

export function WorkflowRunHistoryTable({
  workspaceId,
  productId,
  workflowId,
  runs,
  projectNames,
  memberNames,
  onUpdated,
}: WorkflowRunHistoryTableProps) {
  const router = useRouter();
  const [remarkRun, setRemarkRun] = useState<ProductWorkflowRunSummaryVM | null>(null);
  const [remarkText, setRemarkText] = useState("");
  const [remarkBusy, setRemarkBusy] = useState(false);
  const [remarkError, setRemarkError] = useState<string | null>(null);

  function openRemarkEditor(run: ProductWorkflowRunSummaryVM) {
    setRemarkRun(run);
    setRemarkText(run.remark ?? "");
    setRemarkError(null);
  }

  async function saveRemark() {
    if (!remarkRun) return;
    setRemarkBusy(true);
    setRemarkError(null);
    try {
      await product.updateProductWorkflowRunRemark(workspaceId, productId, workflowId, remarkRun.id, {
        remark: remarkText.trim(),
      });
      setRemarkRun(null);
      onUpdated();
    } catch (err) {
      setRemarkError(err instanceof BffError ? err.message : "保存备注失败");
    } finally {
      setRemarkBusy(false);
    }
  }

  if (runs.length === 0) {
    return <div className="wf-history-empty">暂无执行记录，点击右上角「执行」开始第一次运行。</div>;
  }

  return (
    <>
      <div className="wf-history-table-wrap">
        <table className="wf-history-table">
          <thead>
            <tr>
              <th className="wf-history-col-id">运行</th>
              <th className="wf-history-col-status">状态</th>
              <th className="wf-history-col-executor">执行人</th>
              <th className="wf-history-col-duration">耗时</th>
              <th className="wf-history-col-branch">分支</th>
              <th className="wf-history-col-services">服务组件</th>
              <th className="wf-history-col-remark">备注</th>
            </tr>
          </thead>
          <tbody>
            {runs.map((run) => {
              const runHref = `/products/${productId}/workflows/${workflowId}/runs/${run.id}`;
              const services = serviceGroups(run.build_services, run.deploy_services, projectNames);
              const branches = run.branches ?? [];
              return (
                <tr key={run.id} className="wf-history-row" onClick={() => router.push(runHref)}>
                  <td className="wf-history-col-id">
                    <Link href={runHref} className="wf-history-run-id" onClick={(e) => e.stopPropagation()}>
                      #{run.run_number ?? run.id.slice(0, 8)}
                    </Link>
                  </td>
                  <td className="wf-history-col-status">
                    <span className={`wf-history-status ${runStatusTone(run.status)}`}>
                      <span className="wf-history-status-dot" aria-hidden />
                      {runStatusLabel(run.status)}
                    </span>
                  </td>
                  <td className="wf-history-col-executor">
                    <span className="wf-history-primary">{executorLabel(run, memberNames)}</span>
                    <span className="wf-history-secondary">{formatRunTime(run.started_at ?? run.created_at)}</span>
                  </td>
                  <td className="wf-history-col-duration">
                    <span className="wf-history-numeric">{formatDurationMs(run.duration_ms ?? null)}</span>
                  </td>
                  <td className="wf-history-col-branch">
                    {branches.length > 0 ? (
                      <ul className="wf-history-stack">
                        {branches.map((item, index) => (
                          <li key={`${item.project_id ?? "all"}-${item.branch}-${index}`}>
                            <span className="wf-history-branch">{item.branch}</span>
                            {item.project_id ? (
                              <span className="wf-history-secondary" title={item.project_id}>
                                {projectLabel(item.project_id, projectNames)}
                              </span>
                            ) : null}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <span className="wf-history-muted">{branchSummary(run)}</span>
                    )}
                  </td>
                  <td className="wf-history-col-services">
                    {services.length > 0 ? (
                      <ul className="wf-history-stack">
                        {services.map((item) => (
                          <li key={item.id} className="wf-history-service">
                            <span
                              className={`wf-history-primary${item.known ? "" : " is-unknown"}`}
                              title={item.id}
                            >
                              {item.label}
                            </span>
                            <span className="wf-history-kinds">{item.kinds.join(" · ")}</span>
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <span className="wf-history-muted">—</span>
                    )}
                  </td>
                  <td className="wf-history-col-remark">
                    <div className="wf-history-remark">
                      {run.remark ? (
                        <span className="wf-history-remark-text">{run.remark}</span>
                      ) : null}
                      <button
                        type="button"
                        className={`wf-history-remark-edit${run.remark ? "" : " is-empty"}`}
                        title={run.remark ? "编辑备注" : "添加备注"}
                        aria-label={run.remark ? "编辑备注" : "添加备注"}
                        onClick={(e) => {
                          e.stopPropagation();
                          openRemarkEditor(run);
                        }}
                      >
                        <IconPencil />
                        {run.remark ? null : <span>添加备注</span>}
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <Modal
        open={remarkRun !== null}
        onClose={() => {
          if (!remarkBusy) setRemarkRun(null);
        }}
        title="编辑备注"
        description={remarkRun ? `运行 #${remarkRun.run_number ?? remarkRun.id.slice(0, 8)}` : undefined}
        size="md"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" disabled={remarkBusy} onClick={() => setRemarkRun(null)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={remarkBusy} onClick={() => void saveRemark()}>
              {remarkBusy ? "保存中…" : "保存"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {remarkError ? <InlineAlert tone="error">{remarkError}</InlineAlert> : null}
          <textarea
            className="input wf-run-remark-input"
            rows={4}
            maxLength={500}
            value={remarkText}
            onChange={(e) => setRemarkText(e.target.value)}
            placeholder="记录本次执行说明，便于后续追溯"
            autoFocus
          />
        </div>
      </Modal>
    </>
  );
}
