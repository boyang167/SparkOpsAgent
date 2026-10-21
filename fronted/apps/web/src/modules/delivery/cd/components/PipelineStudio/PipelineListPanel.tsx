"use client";

import Link from "next/link";
import { cloneMockPipelines } from "../../mock/data";

export function PipelineListPanel() {
  const pipelines = cloneMockPipelines();

  return (
    <section className="section stack-md">
      <div className="section-header">
        <div>
          <h2 className="section-title">Pipeline 模板</h2>
          <p className="muted text-sm">配置发布编排 DAG —「怎么发」</p>
        </div>
      </div>
      <table className="data-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>状态</th>
            <th>版本</th>
            <th>Stage 数</th>
            <th>更新时间</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {pipelines.map((pipeline) => (
            <tr key={pipeline.id}>
              <td>
                <strong>{pipeline.name}</strong>
                <p className="muted text-sm">{pipeline.description}</p>
              </td>
              <td>
                <span className={`badge ${pipeline.status === "active" ? "badge-success" : "badge-neutral"}`}>
                  {pipeline.status}
                </span>
              </td>
              <td>v{pipeline.version}</td>
              <td>{pipeline.definition.spec.stages.length}</td>
              <td>{new Date(pipeline.updatedAt).toLocaleString()}</td>
              <td>
                <Link href={`/delivery/pipelines/${pipeline.id}/studio`} className="link-inline">
                  打开 Studio
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
