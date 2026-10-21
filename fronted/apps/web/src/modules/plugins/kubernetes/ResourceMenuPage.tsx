"use client";

import { ResourceTable } from "./ResourceTable";
import { resourcesForMenu } from "./catalog";
import { NodesPanel } from "./specialized/NodesPanel";
import { AccessControlsPanel } from "./specialized/AccessControlsPanel";
import { ConfigsPanel } from "./specialized/ConfigsPanel";
import { NamespacesPanel } from "./specialized/NamespacesPanel";
import { NetworksPanel } from "./specialized/NetworksPanel";
import { StoragesPanel } from "./specialized/StoragesPanel";
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
      {menu === "nodes" || menu === "workloads" || menu === "namespaces" || menu === "networks" || menu === "configs" || menu === "storages" || menu === "access-controls" ? null : (
        <section className="card stack-sm p-4">
          <div>
            <h1 className="text-xl font-semibold">{title}</h1>
            {description ? <p className="text-sm text-muted mt-1">{description}</p> : null}
          </div>
        </section>
      )}
      {menu === "nodes" ? (
        <NodesPanel clusterId={clusterId} canManage={canManage} />
      ) : menu === "workloads" ? (
        <WorkloadsPanel clusterId={clusterId} canManage={canManage} />
      ) : menu === "namespaces" ? (
        <NamespacesPanel clusterId={clusterId} canManage={canManage} />
      ) : menu === "networks" ? (
        <NetworksPanel clusterId={clusterId} canManage={canManage} />
      ) : menu === "configs" ? (
        <ConfigsPanel clusterId={clusterId} canManage={canManage} />
      ) : menu === "storages" ? (
        <StoragesPanel clusterId={clusterId} canManage={canManage} />
      ) : menu === "access-controls" ? (
        <AccessControlsPanel clusterId={clusterId} canManage={canManage} />
      ) : (
        <ResourceTable clusterId={clusterId} resources={resourcesForMenu(menu, canManage)} canManage={canManage} />
      )}
    </div>
  );
}
