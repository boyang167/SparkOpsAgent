"use client";

import { useMemo, useState } from "react";
import { ClusterPicker, type KubernetesCluster } from "./ClusterPicker";
import { ResourceMenuPage } from "./ResourceMenuPage";
import { ChatPage } from "./specialized/ChatPage";
import { CloudTTYPage } from "./specialized/CloudTTYPage";
import { ClustersPanel } from "./specialized/ClustersPanel";
import { CreateDeploymentPage } from "./specialized/CreateDeploymentPage";
import { DiagnosisPage } from "./specialized/DiagnosisPage";
import { OverviewPanel } from "./specialized/OverviewPanel";
import { RecordsPage } from "./specialized/RecordsPage";
import { VelauxPage } from "./specialized/VelauxPage";

export type KubernetesConsoleMode =
  | "overview"
  | "clusters"
  | "resource"
  | "create"
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
  const [clusters, setClusters] = useState<KubernetesCluster[]>([]);

  const body = useMemo(() => {
    if (mode === "overview") return <OverviewPanel />;
    if (mode === "clusters") return <ClustersPanel clusters={clusters} />;
    if (mode === "create") return <CreateDeploymentPage clusterId={clusterId} canManage={canManage} />;
    if (mode === "records") return <RecordsPage canManage={canManage} />;
    if (mode === "diagnosis") return <DiagnosisPage clusterId={clusterId} canManage={canManage} />;
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
  }, [canManage, clusterId, clusters, description, menu, mode, title]);

  return (
    <div className="page stack-lg">
      <ClusterPicker
        value={clusterId}
        onChange={setClusterId}
        onClustersChange={setClusters}
        canManage={canManage}
        showSettings={mode === "clusters"}
      />
      {mode === "resource" ? body : (
        <section className="card stack-md p-4">
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
          </div>
          {body}
        </section>
      )}
    </div>
  );
}
