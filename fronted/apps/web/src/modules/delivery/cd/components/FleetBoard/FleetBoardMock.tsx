"use client";

import Link from "next/link";
import { useMemo } from "react";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { getReleaseRun } from "../../lib/release-run-store";

interface FleetBoardMockProps {
  runId: string;
}

export function FleetBoardMock({ runId }: FleetBoardMockProps) {
  const run = useMemo(() => getReleaseRun(runId), [runId]);

  return (
    <div className="stack-lg">
      {run ? (
        <InlineAlert tone="success">
          发布 Run 已接入向导。
          {run.source === "artifact"
            ? "本次使用已有镜像，未触发 Jenkins。"
            : "本次先构建再发布，已创建真实 DeploymentRun。"}
        </InlineAlert>
      ) : (
        <InlineAlert tone="info">
          未找到本次浏览器会话中的发布记录。Run ID：<code>{runId}</code>
        </InlineAlert>
      )}

      <div className="section">
        <h2 className="section-title">执行概览</h2>
        {run ? (
          <dl className="cd-summary-list">
            <div>
              <dt>项目</dt>
              <dd>{run.projectName}</dd>
            </div>
            <div>
              <dt>制品来源</dt>
              <dd>{run.source === "artifact" ? "使用已有镜像" : "先构建再发布"}</dd>
            </div>
            <div>
              <dt>制品</dt>
              <dd>
                {run.imageRepository}:{run.imageTag}
              </dd>
            </div>
            <div>
              <dt>Pipeline</dt>
              <dd>{run.pipelineName}</dd>
            </div>
            <div>
              <dt>发布策略</dt>
              <dd>{run.strategies.join("；") || "—"}</dd>
            </div>
            <div>
              <dt>Fleet</dt>
              <dd>{run.fleetNames.join(", ") || "—"}</dd>
            </div>
            {run.skippedBuildSteps ? (
              <div>
                <dt>构建步骤</dt>
                <dd>已跳过</dd>
              </div>
            ) : null}
            {run.deploymentRunId ? (
              <div>
                <dt>构建 Run</dt>
                <dd>
                  <code>{run.deploymentRunId}</code>
                </dd>
              </div>
            ) : null}
          </dl>
        ) : (
          <p className="muted text-sm">DAG 进度与集群热力图尚未接入 Run Engine。</p>
        )}
        <div className="cd-inline-row" style={{ marginTop: "1rem" }}>
          <Link href="/delivery/pipelines" className="link-inline">
            Pipeline Studio
          </Link>
          {run ? (
            <Link href={`/projects/${run.projectId}`} className="link-inline">
              返回项目
            </Link>
          ) : null}
        </div>
      </div>
    </div>
  );
}
