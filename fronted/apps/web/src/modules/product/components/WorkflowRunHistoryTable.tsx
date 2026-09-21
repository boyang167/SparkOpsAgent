"use client";

import Link from "next/link";
import { useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductWorkflowRunSummaryVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { formatDurationMs } from "./workflow-list-utils";
import {
  branchSummary,
  executorLabel,
  formatRunTime,
  projectLabel,
  runStatusLabel,
  runStatusTone,
  serviceLines,
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

export function WorkflowRunHistoryTable({
  workspaceId,
  productId,
  workflowId,
  runs,
  projectNames,
  memberNames,
  onUpdated,
}: WorkflowRunHistoryTableProps) {
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
    return <div className="wf-history-empty">暂无执行记录，点击「执行」开始第一次运行。</div>;
  }

  return (
    <>
      <div className="wf-history-table-wrap">
        <table className="wf-history-table">
          <thead>
            <tr>
              <th className="wf-history-col-id">ID</th>
              <th className="wf-history-col-status">运行状态</th>
              <th className="wf-history-col-duration">持续时间</th>
              <th className="wf-history-col-executor">执行人</th>
              <th className="wf-history-col-branch">分支信息</th>
              <th className="wf-history-col-remark">备注</th>
              <th className="wf-history-col-build">服务组件 (build)</th>
              <th className="wf-history-col-deploy">服务组件 (deploy)</th>
            </tr>
          </thead>
          <tbody>
            {runs.map((run) => {
              const runHref = `/products/${productId}/workflows/${workflowId}/runs/${run.id}`;
              const buildLines = serviceLines(run.build_services, projectNames);
              const deployLines = serviceLines(run.deploy_services, projectNames);
              const branches = run.branches ?? [];
              return (
                <tr key={run.id}>
                  <td className="wf-history-col-id">
                    <Link href={runHref} className="wf-history-run-id">
                      #{run.run_number ?? run.id.slice(0, 8)}
                    </Link>
                  </td>
                  <td className="wf-history-col-status">
                    <span className={`wf-history-status ${runStatusTone(run.status)}`}>
                      {runStatusLabel(run.status)}
                    </span>
                  </td>
                  <td className="wf-history-col-duration">{formatDurationMs(run.duration_ms ?? null)}</td>
                  <td className="wf-history-col-executor">
                    <div className="wf-history-executor">
                      <span className="wf-history-executor-name">{executorLabel(run, memberNames)}</span>
                      <span className="wf-history-executor-time">
                        {formatRunTime(run.started_at ?? run.created_at)}
                      </span>
                    </div>
                  </td>
                  <td className="wf-history-col-branch">
                    {branches.length > 0 ? (
                      <div className="wf-history-branch-list">
                        {branches.map((item, index) => (
                          <div key={`${item.project_id ?? "all"}-${item.branch}-${index}`} className="wf-history-branch-row">
                            <span className="wf-history-branch-name">{item.branch}</span>
                            {item.project_id ? (
                              <span className="wf-history-branch-project">
                                {projectLabel(item.project_id, projectNames)}
                              </span>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <span className="wf-history-muted">{branchSummary(run)}</span>
                    )}
                  </td>
                  <td className="wf-history-col-remark">
                    <div className="wf-history-remark-cell">
                      {run.remark ? <span className="wf-history-remark-text">{run.remark}</span> : null}
                      <button
                        type="button"
                        className="wf-history-remark-edit"
                        title="编辑备注"
                        aria-label="编辑备注"
                        onClick={() => openRemarkEditor(run)}
                      >
                        ✎
                      </button>
                    </div>
                  </td>
                  <td className="wf-history-col-build">
                    {buildLines.length > 0 ? (
                      <div className="wf-history-service-list">
                        {buildLines.map((line) => (
                          <div key={line}>{line}</div>
                        ))}
                      </div>
                    ) : (
                      <span className="wf-history-muted">—</span>
                    )}
                  </td>
                  <td className="wf-history-col-deploy">
                    {deployLines.length > 0 ? (
                      <div className="wf-history-service-list">
                        {deployLines.map((line) => (
                          <div key={line}>{line}</div>
                        ))}
                      </div>
                    ) : (
                      <span className="wf-history-muted">—</span>
                    )}
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
        description={remarkRun ? `#${remarkRun.run_number ?? remarkRun.id.slice(0, 8)}` : undefined}
        size="md"
        footer={
          <>
            <button type="button" className="btn btn-primary text-sm" disabled={remarkBusy} onClick={() => void saveRemark()}>
              {remarkBusy ? "保存中…" : "保存"}
            </button>
            <button type="button" className="btn btn-ghost text-sm" disabled={remarkBusy} onClick={() => setRemarkRun(null)}>
              取消
            </button>
          </>
        }
      >
        <div className="stack-sm">
          {remarkError ? <p className="text-sm text-red-600">{remarkError}</p> : null}
          <textarea
            className="input wf-run-remark-input"
            rows={4}
            maxLength={500}
            value={remarkText}
            onChange={(e) => setRemarkText(e.target.value)}
            placeholder="记录本次执行说明"
          />
        </div>
      </Modal>
    </>
  );
}
