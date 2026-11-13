import { redirect, notFound } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { FileExplorerPanel } from "@/modules/filesystem/components/FileExplorerPanel";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadFileNamespace, loadFileNodes, loadFileRevisions } from "@/server/loaders/filesystem";

export default async function FileNodePage({
  params,
  searchParams,
}: {
  params: Promise<{ nodeId: string }>;
  searchParams: Promise<{ mode?: string }>;
}) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { nodeId } = await params;
  const { mode } = await searchParams;
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
    notFound();
  }

  if (mode !== "revisions") {
    const children = await loadFileNodes(workspaceId, nodeId);
    return (
      <div className="stack-lg">
        <PageHeader eyebrow="Filesystem" title="目录" description={`节点 ${nodeId}`} />
        <FileExplorerPanel
          workspaceId={workspaceId}
          namespaceId={namespace.id}
          parentId={nodeId}
          initialNodes={children.items}
          breadcrumb={[{ id: nodeId, name: "…" }]}
        />
      </div>
    );
  }

  const revisions = await loadFileRevisions(workspaceId, nodeId);

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Filesystem" title="文件版本" description={`节点 ${nodeId}`} />
      <div className="card">
        {revisions.items.length === 0 ? (
          <p className="text-muted">暂无版本记录。</p>
        ) : (
          <table className="data-table w-full">
            <thead>
              <tr>
                <th>版本</th>
                <th>大小</th>
                <th>MIME</th>
              </tr>
            </thead>
            <tbody>
              {revisions.items.map((revision) => (
                <tr key={revision.id}>
                  <td>#{revision.revision_no}</td>
                  <td>{revision.size}</td>
                  <td className="text-muted">{revision.mime_type}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
