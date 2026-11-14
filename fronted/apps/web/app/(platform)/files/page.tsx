import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { FilesystemWorkbench } from "@/modules/filesystem/components/FilesystemWorkbench";
import { resolveWorkbenchPermissions } from "@/modules/filesystem/components/workbench-permissions";
import { parseWorkbenchState } from "@/modules/filesystem/components/workbench-state";
import { loadSession } from "@/server/loaders/session";
import { loadMyPermissions } from "@/server/loaders/governance";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import {
  loadFileNamespace,
  loadFileNode,
  loadFileNodes,
  resolveFileNodeSelection,
} from "@/server/loaders/filesystem";

export default async function FilesPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ??
    (await loadWorkspaceId()) ??
    workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  let workspacePermissions: string[] | undefined;
  try {
    workspacePermissions = await loadMyPermissions(workspaceId);
  } catch {
    workspacePermissions = undefined;
  }
  const permissions = resolveWorkbenchPermissions(
    workspacePermissions,
    session.is_super_admin ?? false,
  );

  const namespace = await loadFileNamespace(workspaceId);
  const initialState = parseWorkbenchState(await searchParams);
  const rootId = namespace.root_node_id;
  if (!rootId) {
    return (
      <div className="stack-lg">
        <PageHeader
          eyebrow="Filesystem"
          title="文件"
          description="工作区文件命名空间尚未初始化。"
        />
      </div>
    );
  }

  const listing = await loadFileNodes(workspaceId, initialState.parent ?? rootId);
  const selection = initialState.node
    ? await resolveFileNodeSelection(() => loadFileNode(workspaceId, initialState.node!))
    : {};

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Filesystem" title="文件" description="浏览与管理 workspace 文件与目录" />
      <FilesystemWorkbench
        workspaceId={workspaceId}
        namespace={namespace}
        initialNodes={listing.items}
        initialState={initialState}
        permissions={permissions}
        initialSelectedNode={selection.node}
        initialSelectedNodeError={selection.error}
      />
    </div>
  );
}
