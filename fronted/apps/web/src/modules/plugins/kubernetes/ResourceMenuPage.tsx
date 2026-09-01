"use client";

import { ResourceTable } from "./ResourceTable";
import { resourcesForMenu } from "./catalog";
import { NodesPanel } from "./specialized/NodesPanel";
import { WorkloadsPanel } from "./specialized/WorkloadsPanel";

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
      {menu === "nodes" || menu === "workloads" ? null : (
        <section className="card stack-sm p-4">
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
          </div>
        </section>
      )}
      {menu === "nodes" ? <NodesPanel clusterId={clusterId} canManage={canManage} /> : menu === "workloads" ? (
        <WorkloadsPanel clusterId={clusterId} canManage={canManage} />
      ) : (
        <ResourceTable clusterId={clusterId} resources={resourcesForMenu(menu, canManage)} canManage={canManage} />
      )}
    </div>
  );
}
