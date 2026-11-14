import { redirect } from "next/navigation";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PageHeader } from "@/components/ui/PageHeader";
import { FileSpaceNav } from "@/modules/filesystem/components/FileSpaceNav";
import { FilesystemWorkbench } from "@/modules/filesystem/components/FilesystemWorkbench";
import { SharedWithMePanel } from "@/modules/filesystem/components/SharedWithMePanel";
import { resolveFileSpaceView } from "@/modules/filesystem/components/file-space-view";
import {
  hasFilesystemPageAccess,
  resolveWorkbenchPermissions,
} from "@/modules/filesystem/components/workbench-permissions";
import { loadSession } from "@/server/loaders/session";
import { loadMyPermissions } from "@/server/loaders/governance";
import { loadMembers } from "@/server/loaders/governance";
import { loadTenantContext } from "@/server/loaders/tenant-context";
import { loadMyOrganizationPermissions } from "@/server/loaders/organization-permissions";
import {
  loadAccessibleFileNamespaces,
  loadGovernedFileChildren,
  loadFileNode,
  loadSharedFileItems,
  resolveFileSpaceLoad,
  resolveFileNodeSelection,
} from "@/server/loaders/filesystem";
import { normalizeFilesPageSearch } from "./page-state";

export default async function FilesPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }
  const normalizedSearch = normalizeFilesPageSearch(await searchParams);
  if (normalizedSearch.redirectPath) {
    redirect(normalizedSearch.redirectPath);
  }

  const tenantContext = await loadTenantContext();
  const { workspaceId, currentWorkspace, currentOrganization } = tenantContext;
  if (!workspaceId) {
    redirect("/dashboard");
  }
  const initialState = normalizedSearch.state;
  if (
    !currentWorkspace ||
    currentWorkspace.is_member === false ||
    !currentOrganization
  ) {
    return (
      <div className="stack-lg">
        <PageHeader
          eyebrow="Filesystem"
          title="文件中心"
          description="当前工作区上下文不可用。"
        />
        <InlineAlert tone="error">
          你尚未加入当前工作区，或该工作区不属于可访问的租户。
        </InlineAlert>
      </div>
    );
  }

  let workspacePermissions: string[] | undefined;
  try {
    workspacePermissions = await loadMyPermissions(workspaceId);
  } catch {
    workspacePermissions = undefined;
  }
  let organizationPermissions: string[] = [];
  try {
    organizationPermissions = await loadMyOrganizationPermissions(currentOrganization.id);
  } catch {
    organizationPermissions = [];
  }
  const permissions = resolveWorkbenchPermissions(
    workspacePermissions,
    session.is_super_admin ?? false,
    organizationPermissions,
  );
  let members: WorkspaceMemberVM[] = [];
  let memberDirectoryAvailable = false;
  try {
    members = await loadMembers(workspaceId);
    memberDirectoryAvailable = true;
  } catch {
    members = [];
  }
  if (!members.some((member) => member.member_id === session.id)) {
    members.push({
      member_id: session.id,
      email: session.email,
      display_name: session.display_name,
      roles: [],
    });
  }

  if (!hasFilesystemPageAccess(permissions)) {
    return (
      <div className="stack-lg">
        <PageHeader
          eyebrow="Filesystem"
          title="文件中心"
          description="当前角色没有访问工作区文件的权限。"
        />
        <InlineAlert tone="error">
          无权访问文件中心。请联系工作区管理员授予文件读取权限。
        </InlineAlert>
      </div>
    );
  }

  const organizationLabel = currentOrganization.name;
  const workspaceLabel = currentWorkspace.name;
  const namespaceLoad = await resolveFileSpaceLoad(() =>
    loadAccessibleFileNamespaces(currentOrganization.id, workspaceId),
  );
  if (namespaceLoad.kind !== "loaded") {
    const message =
      namespaceLoad.kind === "forbidden"
        ? "无权读取当前租户的文件空间。"
        : namespaceLoad.kind === "not-found"
          ? "当前租户的文件空间不存在。"
          : "文件空间加载失败，请稍后重试。";
    return (
      <div className="stack-lg">
        <PageHeader eyebrow="Filesystem" title="文件中心" description="浏览已授权的文件空间。" />
        <FileSpaceNav
          activeSpace={initialState.space}
          organizationLabel={organizationLabel}
          workspaceLabel={workspaceLabel}
          workspaceId={workspaceId}
          view={initialState.view}
        />
        <InlineAlert tone="error">{message}</InlineAlert>
      </div>
    );
  }

  const spaceView = resolveFileSpaceView({
    space: initialState.space,
    summaries: namespaceLoad.value.items,
    organizationId: currentOrganization.id,
    workspaceId,
    userId: session.id,
    workspaceIsMember: true,
  });
  if (spaceView.kind === "workspace-unavailable" || spaceView.kind === "missing") {
    const message =
      spaceView.kind === "workspace-unavailable"
        ? "当前工作区不可用，无法解析文件空间。"
        : `${{
            my: "个人",
            workspace: "工作区",
            organization: "租户",
          }[spaceView.space]}文件空间尚未创建或未向你授权。`;
    return (
      <div className="stack-lg">
        <PageHeader eyebrow="Filesystem" title="文件中心" description="浏览已授权的文件空间。" />
        <FileSpaceNav
          activeSpace={initialState.space}
          organizationLabel={organizationLabel}
          workspaceLabel={workspaceLabel}
          workspaceId={workspaceId}
          view={initialState.view}
        />
        <InlineAlert tone="error">{message}</InlineAlert>
      </div>
    );
  }

  if (spaceView.kind === "shared") {
    const sharedLoad = await resolveFileSpaceLoad(() =>
      loadSharedFileItems(currentOrganization.id, workspaceId, {
        source: initialState.source,
        cursor: initialState.cursor,
        query: initialState.query,
        limit: 100,
      }),
    );
    const sharedItems = sharedLoad.kind === "loaded" ? sharedLoad.value.items : [];
    const sharedState =
      sharedLoad.kind === "loaded"
        ? sharedItems.length > 0
          ? "ready"
          : "empty"
        : sharedLoad.kind === "forbidden"
          ? "forbidden"
          : "error";
    return (
      <div className="stack-lg">
        <PageHeader eyebrow="Filesystem" title="文件中心" description="浏览已授权的文件空间。" />
        <FileSpaceNav
          activeSpace="shared"
          organizationLabel={organizationLabel}
          workspaceLabel={workspaceLabel}
          workspaceId={workspaceId}
          view={initialState.view}
        />
        <SharedWithMePanel
          workspaceId={workspaceId}
          organizationId={currentOrganization.id}
          organizationLabel={organizationLabel}
          workspaceLabel={workspaceLabel}
          source={initialState.source}
          items={sharedItems}
          state={sharedState}
          members={members}
          memberDirectoryAvailable={memberDirectoryAvailable}
        />
      </div>
    );
  }

  const namespaceSummary = spaceView.namespace;
  const rootId = namespaceSummary.root_node_id;
  if (!rootId) {
    return (
      <div className="stack-lg">
        <PageHeader
          eyebrow="Filesystem"
          title="文件中心"
          description="所选文件空间尚未初始化。"
        />
        <FileSpaceNav
          activeSpace={initialState.space}
          organizationLabel={organizationLabel}
          workspaceLabel={workspaceLabel}
          namespace={namespaceSummary}
          workspaceId={workspaceId}
          view={initialState.view}
        />
      </div>
    );
  }

  const listingLoad = await resolveFileSpaceLoad(() =>
    loadGovernedFileChildren(
      currentOrganization.id,
      workspaceId,
      namespaceSummary.id,
      {
      parentId: initialState.parent ?? rootId,
      cursor: initialState.cursor,
      query: initialState.query,
      limit: 100,
      },
    ),
  );
  if (listingLoad.kind !== "loaded") {
    const message =
      listingLoad.kind === "forbidden"
        ? "无权访问所选目录。"
        : listingLoad.kind === "not-found"
          ? "所选目录不存在或已被移除。"
          : "目录内容加载失败，请稍后重试。";
    return (
      <div className="stack-lg">
        <PageHeader eyebrow="Filesystem" title="文件中心" description="浏览已授权的文件空间。" />
        <FileSpaceNav
          activeSpace={initialState.space}
          organizationLabel={organizationLabel}
          workspaceLabel={workspaceLabel}
          namespace={namespaceSummary}
          workspaceId={workspaceId}
          view={initialState.view}
        />
        <InlineAlert tone="error">{message}</InlineAlert>
      </div>
    );
  }
  const listing = listingLoad.value;
  const selection = initialState.node
    ? await resolveFileNodeSelection(
        namespaceSummary.id,
        () => loadFileNode(workspaceId, initialState.node!),
      )
    : {};
  const namespace = {
    id: namespaceSummary.id,
    workspace_id: workspaceId,
    root_node_id: namespaceSummary.root_node_id ?? null,
    status: namespaceSummary.status,
  };

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Filesystem" title="文件中心" description="浏览与管理已授权的文件与目录。" />
      <FilesystemWorkbench
        workspaceId={workspaceId}
        namespace={namespace}
        initialNodes={listing.items}
        initialNextCursor={listing.next_cursor}
        initialState={initialState}
        permissions={permissions}
        initialSelectedNode={selection.node}
        initialSelectedNodeError={selection.error}
        fileSpace={initialState.space}
        namespaceSummary={namespaceSummary}
        organizationLabel={organizationLabel}
        workspaceLabel={workspaceLabel}
        organizationId={currentOrganization.id}
        members={members}
        memberDirectoryAvailable={memberDirectoryAvailable}
      />
    </div>
  );
}
