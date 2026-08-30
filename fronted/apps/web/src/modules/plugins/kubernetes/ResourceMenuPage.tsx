"use client";

import { ResourceTable } from "./ResourceTable";
import { resourcesForMenu } from "./catalog";
import { OperationsPanel } from "./OperationsPanel";

type ResourceMenuPageProps = {
  title: string;
  description?: string;
  menu: string;
  clusterId: string;
  canManage: boolean;
};

export function ResourceMenuPage({ title, description, menu, clusterId, canManage }: ResourceMenuPageProps) {
  return (
    <div className="stack-md">
      <section className="card stack-sm p-4">
        <div>
          <h1 className="text-xl font-semibold">{title}</h1>
          {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
        </div>
      </section>
      <ResourceTable clusterId={clusterId} resources={resourcesForMenu(menu, canManage)} canManage={canManage} />
      {menu === "workloads" ? <OperationsPanel clusterId={clusterId} canManage={canManage} mode="pods" /> : null}
      {menu === "nodes" ? <OperationsPanel clusterId={clusterId} canManage={canManage} mode="nodes" /> : null}
      {menu === "nodes" || menu === "workloads" ? (
        <OperationsPanel clusterId={clusterId} canManage={canManage} mode="metrics" />
      ) : null}
    </div>
  );
}
