"use client";

import { useMemo, useState } from "react";
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
    return (
      <div className="page stack-lg">
        <section className="card stack-md p-4">
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
          </div>
          <ClusterPicker value={clusterId} onChange={setClusterId} canManage={canManage} layout="table" />
        </section>
      </div>
    );
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
        <DiagnosisPanel canManage={canManage} />
      </div>
    );
  }

  return (
    <div className="page stack-lg">
      <ClusterPicker value={clusterId} onChange={setClusterId} canManage={canManage} layout="picker" />
      {mode === "resource" ? (
        body
      ) : (
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
