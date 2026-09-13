"use client";

import { K8sEmptyState, K8sPanelCard } from "@/components/k8s";
import { buildVelauxFrameProps } from "../velaux-frame";

export function VelauxPage({ clusterId }: { clusterId: string }) {
  if (!clusterId) {
    return (
      <K8sPanelCard>
        <K8sEmptyState variant="no-cluster" />
      </K8sPanelCard>
    );
  }
  const frame = buildVelauxFrameProps(clusterId);
  return (
    <K8sPanelCard className="stack-sm">
      <div className="alert alert-info">
        Velaux 仅在隔离 iframe 内通过同源 BFF proxy 打开，不提供未沙箱化的新标签入口，也不暴露后端配置 URL 或密钥。
      </div>
      <iframe
        title="Velaux"
        src={frame.src}
        className="w-full min-h-[640px] rounded-md border"
        sandbox={frame.sandbox}
        referrerPolicy={frame.referrerPolicy}
      />
    </K8sPanelCard>
  );
}
