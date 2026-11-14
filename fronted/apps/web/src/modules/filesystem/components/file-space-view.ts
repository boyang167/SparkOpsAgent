import type { NamespaceSummaryVM } from "@sparkops/contracts/filesystem";
import type { FileSpace } from "./workbench-state";

export type FileSpaceView =
  | { kind: "namespace"; space: Exclude<FileSpace, "shared">; namespace: NamespaceSummaryVM }
  | { kind: "shared"; space: "shared" }
  | { kind: "missing"; space: Exclude<FileSpace, "shared"> }
  | { kind: "workspace-unavailable"; space: FileSpace };

export function resolveFileSpaceView(input: {
  space: FileSpace;
  summaries: NamespaceSummaryVM[];
  organizationId?: string;
  workspaceId?: string;
  userId: string;
  workspaceIsMember: boolean;
}): FileSpaceView {
  const {
    space,
    summaries,
    organizationId,
    workspaceId,
    userId,
    workspaceIsMember,
  } = input;
  if (!organizationId || !workspaceId || !workspaceIsMember) {
    return { kind: "workspace-unavailable", space };
  }
  if (space === "shared") return { kind: "shared", space };

  const namespaceType = {
    my: "USER",
    workspace: "WORKSPACE",
    organization: "ORGANIZATION",
  }[space] as NamespaceSummaryVM["namespace_type"];
  const namespace = summaries.find((item) => {
    if (
      item.namespace_type !== namespaceType ||
      item.organization_id !== organizationId
    ) {
      return false;
    }
    if (space === "my") return item.owner_user_id === userId;
    if (space === "workspace") return item.workspace_id === workspaceId;
    return true;
  });
  return namespace
    ? { kind: "namespace", space, namespace }
    : { kind: "missing", space };
}
