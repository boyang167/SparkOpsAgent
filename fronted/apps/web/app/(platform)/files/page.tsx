import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { FileExplorerPanel } from "@/modules/filesystem/components/FileExplorerPanel";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadFileNamespace, loadFileNodes } from "@/server/loaders/filesystem";

export default async function FilesPage() {
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

  const namespace = await loadFileNamespace(workspaceId);
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

  const listing = await loadFileNodes(workspaceId, rootId);

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Filesystem" title="文件" description="浏览与管理 workspace 文件与目录" />
      <FileExplorerPanel
        workspaceId={workspaceId}
        namespaceId={namespace.id}
        parentId={rootId}
        initialNodes={listing.items}
      />
    </div>
  );
}
