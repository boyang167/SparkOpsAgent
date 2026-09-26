"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import type { ProductProjectVM } from "@sparkops/contracts/product";
import type { ProjectVersionDetailVM, ProjectVersionListItemVM } from "@sparkops/contracts/workspace";
import { BffError } from "@sparkops/contracts";
import { workspace } from "@sparkops/web-sdk";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";

interface ProductVersionsPanelProps {
  workspaceId: string;
  productId: string;
  projects: ProductProjectVM[];
}

type VersionRow = ProjectVersionListItemVM & {
  projectId: string;
  projectName: string;
};

function formatCreatedAt(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function statusLabel(status: string): string {
  switch (status) {
    case "ready":
      return "成功";
    case "creating":
      return "创建中";
    case "failed":
      return "失败";
    default:
      return status;
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case "ready":
      return "badge badge-success";
    case "failed":
      return "badge badge-danger";
    default:
      return "badge badge-neutral";
  }
}

function imageSummary(images: ProjectVersionListItemVM["images"]): string {
  if (!images.length) return "—";
  return images.map((item) => `${item.image}${item.tag ? `:${item.tag}` : ""}`.trim()).join(" · ");
}

export function ProductVersionsPanel({ workspaceId, productId, projects }: ProductVersionsPanelProps) {
  const [items, setItems] = useState<VersionRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<string | null>(null);
  const [detail, setDetail] = useState<ProjectVersionDetailVM | null>(null);
  const [detailProjectId, setDetailProjectId] = useState<string | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const showProjectColumn = projects.length > 1;

  const load = useCallback(async () => {
    if (projects.length === 0) {
      setItems([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setMessage(null);
    try {
      const results = await Promise.all(
        projects.map(async (project) => {
          const versions = await workspace.listProjectVersions(workspaceId, project.id);
          return versions.map((item) => ({
            ...item,
            projectId: project.id,
            projectName: project.display_name?.trim() || project.name,
          }));
        }),
      );
      const merged = results.flat().sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at));
      setItems(merged);
    } catch (err) {
      setItems([]);
      setMessage(err instanceof BffError ? err.message : "加载版本列表失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, projects]);

  useEffect(() => {
    void load();
  }, [load]);

  const detailTitle = useMemo(() => {
    if (!detail) return "版本详情";
    return `版本 ${detail.version_tag}`;
  }, [detail]);

  async function openDetail(row: VersionRow) {
    setDetailProjectId(row.projectId);
    setDetailLoading(true);
    try {
      setDetail(await workspace.getProjectVersion(workspaceId, row.projectId, row.id));
    } catch (err) {
      setMessage(err instanceof BffError ? err.message : "加载版本详情失败");
    } finally {
      setDetailLoading(false);
    }
  }

  function closeDetail() {
    setDetail(null);
    setDetailProjectId(null);
  }

  if (projects.length === 0) {
    return (
      <ProductDetailEmptyState
        title="暂无关联项目"
        description="绑定项目后，工作流产生的交付版本将在此展示。"
        action={
          <Link href={`/products/${productId}?tab=projects`} className="btn btn-primary text-sm">
            前往项目配置
          </Link>
        }
      />
    );
  }

  return (
    <div className="product-versions-panel">
      {message ? <InlineAlert tone="error">{message}</InlineAlert> : null}

      <div className="product-versions-table-wrap">
        <table className="data-table text-sm product-versions-table">
          <thead>
            <tr>
              <th>版本</th>
              <th>状态</th>
              {showProjectColumn ? <th>项目</th> : null}
              <th>标签</th>
              <th>创建人</th>
              <th>创建时间</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={showProjectColumn ? 6 : 5} className="product-versions-placeholder">
                  加载中…
                </td>
              </tr>
            ) : null}
            {!loading && items.length === 0 ? (
              <tr>
                <td colSpan={showProjectColumn ? 6 : 5} className="product-versions-placeholder">
                  <span>暂无交付版本</span>
                  <span className="product-versions-placeholder-hint">
                    版本在工作流<strong>运行成功</strong>后产生：部署任务选「交付版本」并执行，或运行「创建交付版本」任务。
                    仅保存工作流配置不会产生版本。
                    <Link href={`/products/${productId}?tab=workflows`}>前往工作流运行</Link>
                  </span>
                </td>
              </tr>
            ) : null}
            {!loading
              ? items.map((item) => (
                  <tr key={`${item.projectId}-${item.id}`} onClick={() => void openDetail(item)}>
                    <td>
                      <button type="button" className="product-version-link">
                        {item.version_tag}
                      </button>
                    </td>
                    <td>
                      <span className={statusBadgeClass(item.status)}>{statusLabel(item.status)}</span>
                    </td>
                    {showProjectColumn ? <td className="text-muted">{item.projectName}</td> : null}
                    <td className="text-muted">—</td>
                    <td className="text-muted">—</td>
                    <td>{formatCreatedAt(item.created_at)}</td>
                  </tr>
                ))
              : null}
          </tbody>
        </table>
      </div>

      <Drawer open={detail != null || detailLoading} title={detailTitle} onClose={closeDetail}>
        {detailLoading && !detail ? <p className="text-muted text-sm">加载中…</p> : null}
        {detail ? (
          <div className="stack-md">
            <dl className="product-version-detail-meta">
              <div>
                <dt>状态</dt>
                <dd>
                  <span className={statusBadgeClass(detail.status)}>{statusLabel(detail.status)}</span>
                </dd>
              </div>
              {showProjectColumn && detailProjectId ? (
                <div>
                  <dt>项目</dt>
                  <dd>{projects.find((item) => item.id === detailProjectId)?.display_name || detailProjectId}</dd>
                </div>
              ) : null}
              <div>
                <dt>Git 引用</dt>
                <dd>
                  {detail.git_ref_type} / {detail.git_ref_value}
                </dd>
              </div>
              <div>
                <dt>镜像</dt>
                <dd className="mono text-sm">{imageSummary(detail.images)}</dd>
              </div>
              {detail.workflow_run_id ? (
                <div>
                  <dt>来源 Run</dt>
                  <dd className="mono text-sm">{detail.workflow_run_id}</dd>
                </div>
              ) : null}
            </dl>
            {detail.manifests.map((manifest) => (
              <details key={manifest.kind} open={manifest.kind === "Deployment"}>
                <summary className="text-sm font-medium">{manifest.kind}</summary>
                <pre className="product-version-yaml">{manifest.content}</pre>
              </details>
            ))}
          </div>
        ) : null}
      </Drawer>
    </div>
  );
}
