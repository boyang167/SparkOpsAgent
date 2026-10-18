"use client";

import { useMemo, useRef, useState } from "react";
import { PageHeader } from "@/components/ui/PageHeader";
import { ClusterPicker } from "./ClusterPicker";
import { ResourceMenuPage } from "./ResourceMenuPage";
import { ChatPage } from "./specialized/ChatPage";
import { CloudTTYPage } from "./specialized/CloudTTYPage";
import { DiagnosisPanel } from "./specialized/DiagnosisPanel";
import { OverviewPanel } from "./specialized/OverviewPanel";
import { RecordsPage } from "./specialized/RecordsPage";
import { VelauxPage } from "./specialized/VelauxPage";

export type KubernetesConsoleMode =
  | "overview"
  | "clusters"
  | "resource"
  | "records"
  | "diagnosis"
  | "chat"
  | "cloudtty"
  | "velaux";

type KubernetesConsoleProps = {
  mode: KubernetesConsoleMode;
  title: string;
  description?: string;
  menu?: string;
  canManage: boolean;
};

export function KubernetesConsole({ mode, title, description, menu, canManage }: KubernetesConsoleProps) {
  const [clusterId, setClusterId] = useState("");

  const body = useMemo(() => {
    if (mode === "overview") return <OverviewPanel />;
    if (mode === "diagnosis") return null;
    if (mode === "chat") return <ChatPage clusterId={clusterId} canManage={canManage} />;
    if (mode === "cloudtty") return <CloudTTYPage clusterId={clusterId} canManage={canManage} />;
    if (mode === "velaux") return <VelauxPage clusterId={clusterId} />;
    return (
      <ResourceMenuPage
        title={title}
        description={description}
        menu={menu ?? ""}
        clusterId={clusterId}
        canManage={canManage}
      />
    );
  }, [canManage, clusterId, description, menu, mode, title]);

  if (mode === "clusters") {
    return <KubernetesClustersPage title={title} description={description} canManage={canManage} />;
  }

  if (mode === "records") {
    return (
      <div className="page stack-lg">
        <RecordsPage canManage={canManage} />
      </div>
    );
  }

  if (mode === "diagnosis") {
    return (
      <div className="page stack-lg">
        <PageHeader eyebrow="Kubernetes" title={title} description={description} />
        <DiagnosisPanel canManage={canManage} />
      </div>
    );
  }

  return (
    <div className="page stack-lg">
      {mode === "resource" ? (
        <>
          <PageHeader eyebrow="Kubernetes" title={title} description={description} />
          <ClusterPicker value={clusterId} onChange={setClusterId} canManage={canManage} layout="picker" />
          {body}
        </>
      ) : (
        <>
          <ClusterPicker value={clusterId} onChange={setClusterId} canManage={canManage} layout="picker" />
          <section className="card stack-md p-4">
            <div>
              <h1 className="text-xl font-semibold">{title}</h1>
              {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
            </div>
            {body}
          </section>
        </>
      )}
    </div>
  );
}

function KubernetesClustersPage({
  title,
  description,
  canManage,
}: {
  title: string;
  description?: string;
  canManage: boolean;
}) {
  const [clusterId, setClusterId] = useState("");
  const createClusterRef = useRef<(() => void) | null>(null);

  return (
    <div className="page stack-lg">
      <PageHeader
        eyebrow="Kubernetes"
        title={title}
        description={description}
        actions={
          canManage ? (
            <button
              type="button"
              className="btn btn-primary text-sm"
              onClick={() => createClusterRef.current?.()}
            >
              接入集群
            </button>
          ) : null
        }
      />
      <ClusterPicker
        value={clusterId}
        onChange={setClusterId}
        canManage={canManage}
        layout="table"
        createActionRef={createClusterRef}
      />
    </div>
  );
}
