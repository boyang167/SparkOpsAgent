"use client";

import { useCallback, useEffect, useState } from "react";
import type { ProjectVersionDetailVM, ProjectVersionListItemVM } from "@sparkops/contracts/workspace";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface ProjectVersionsTabProps {
  workspaceId: string;
  projectId: string;
  active: boolean;
}

function imageSummary(images: ProjectVersionListItemVM["images"]): string {
  if (!images.length) return "—";
  return images.map((item) => `${item.service_name}: ${item.image}:${item.tag}`.trim()).join(" · ");
}

export function ProjectVersionsTab({ workspaceId, projectId, active }: ProjectVersionsTabProps) {
  const [items, setItems] = useState<ProjectVersionListItemVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [detail, setDetail] = useState<ProjectVersionDetailVM | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setMessage(null);
    try {
      setItems(await workspace.listProjectVersions(workspaceId, projectId));
    } catch (err) {
      setItems([]);
      setMessage(err instanceof BffError ? err.message : "加载版本列表失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, projectId]);

  useEffect(() => {
    if (!active) return;
    void load();
  }, [active, load]);

  async function openDetail(versionId: string) {
    setDetailLoading(true);
    try {
      setDetail(await workspace.getProjectVersion(workspaceId, projectId, versionId));
    } catch (err) {
      setMessage(err instanceof BffError ? err.message : "加载版本详情失败");
    } finally {
      setDetailLoading(false);
    }
  }

  return (
    <div className="project-versions-tab">
      {message ? <InlineAlert tone="error">{message}</InlineAlert> : null}
      {loading ? <p className="project-versions-muted">加载中…</p> : null}
      {!loading && items.length === 0 ? (
        <p className="project-versions-muted">暂无交付版本。版本由产品工作流中的「创建交付版本」任务产生。</p>
      ) : null}
      {!loading && items.length > 0 ? (
        <table className="project-versions-table">
          <thead>
            <tr>
              <th>版本号</th>
              <th>状态</th>
              <th>镜像</th>
              <th>来源 Run</th>
              <th>创建时间</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className="project-versions-row" onClick={() => void openDetail(item.id)}>
                <td>{item.version_tag}</td>
                <td>{item.status}</td>
                <td className="project-versions-mono">{imageSummary(item.images)}</td>
                <td className="project-versions-mono">{item.workflow_run_id || "—"}</td>
                <td>{item.created_at}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}

      <Drawer
        open={detail != null || detailLoading}
        title={detail ? `版本 ${detail.version_tag}` : "版本详情"}
        onClose={() => setDetail(null)}
        width={720}
      >
        {detailLoading && !detail ? <p>加载中…</p> : null}
        {detail ? (
          <div className="project-versions-detail">
            <p>
              <strong>Git 引用：</strong>
              {detail.git_ref_type} / {detail.git_ref_value}
            </p>
            <p>
              <strong>镜像：</strong>
              {imageSummary(detail.images)}
            </p>
            {detail.manifests.map((manifest) => (
              <details key={manifest.kind} open={manifest.kind === "Deployment"}>
                <summary>{manifest.kind}</summary>
                <pre className="project-versions-yaml">{manifest.content}</pre>
              </details>
            ))}
          </div>
        ) : null}
      </Drawer>
    </div>
  );
}
