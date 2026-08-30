"use client";

export function VelauxPage({ clusterId }: { clusterId: string }) {
  if (!clusterId) return <div className="empty-state">请先选择集群。</div>;
  const src = `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/velaux/`;
  return (
    <div className="stack-sm">
      <div className="alert alert-info">
        Velaux 通过同源 BFF proxy 打开。iframe 已隔离，不直接暴露后端配置 URL 或密钥；
        <a className="link-primary ml-1" href={src} target="_blank" rel="noreferrer">
          新标签打开同源入口
        </a>
        。
      </div>
      <iframe
        title="Velaux"
        src={src}
        className="w-full min-h-[640px] rounded-md border"
        sandbox="allow-scripts allow-forms allow-popups allow-downloads"
        referrerPolicy="no-referrer"
      />
    </div>
  );
}
