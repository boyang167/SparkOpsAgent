"use client";

import { useCallback, useEffect, useState } from "react";
import { delivery } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { CronJobVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

interface CronJobsPanelProps {
  workspaceId: string;
  projects: ProjectVM[];
  initialProjectId?: string;
}

export function CronJobsPanel({ workspaceId, projects, initialProjectId }: CronJobsPanelProps) {
  const [projectId, setProjectId] = useState(initialProjectId ?? projects[0]?.id ?? "");
  const [items, setItems] = useState<CronJobVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [schedule, setSchedule] = useState("0 * * * *");
  const [branch, setBranch] = useState("main");
  const { confirm, ConfirmDialog } = useConfirmDialog();

  const load = useCallback(async () => {
    if (!projectId) {
      setItems([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      setItems(await delivery.listCronJobs(workspaceId, projectId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, projectId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!projectId) return;
    try {
      await delivery.createCronJob(workspaceId, projectId, { name, schedule, branch });
      setOpen(false);
      setName("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    }
  }

  async function onDelete(id: string) {
    if (!projectId) return;
    const ok = await confirm({ title: "确定删除该定时任务？", danger: true });
    if (!ok) return;
    try {
      await delivery.deleteCronJob(workspaceId, projectId, id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    }
  }

  return (
    <>
    <div className="stack-md">
      <div className="flex flex-wrap gap-3 justify-between">
        <select className="input" value={projectId} onChange={(e) => setProjectId(e.target.value)}>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <button type="button" className="btn" onClick={() => setOpen(true)} disabled={!projectId}>新建 CronJob</button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无定时构建任务</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>Schedule</th>
                <th>分支</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={row.id}>
                  <td>{row.name}</td>
                  <td className="font-mono text-xs">{row.schedule}</td>
                  <td>{row.branch ?? "main"}</td>
                  <td>
                    <button type="button" className="text-xs text-red-600 hover:underline" onClick={() => void onDelete(row.id)}>
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="新建定时构建">
        <form onSubmit={onCreate} className="stack-md">
          <input className="input" placeholder="名称" value={name} onChange={(e) => setName(e.target.value)} required />
          <input className="input" placeholder="Cron 表达式" value={schedule} onChange={(e) => setSchedule(e.target.value)} required />
          <input className="input" placeholder="分支" value={branch} onChange={(e) => setBranch(e.target.value)} />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setOpen(false)}>取消</button>
            <button type="submit" className="btn">创建</button>
          </div>
        </form>
      </Modal>
    </div>
    <ConfirmDialog />
    </>
  );
}
