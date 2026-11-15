import { redirect } from "next/navigation";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { FileSpaceNav } from "@/modules/filesystem/components/FileSpaceNav";
import { FileSpaceTabList } from "@/modules/filesystem/components/FileSpaceTabList";
import { FilesystemWorkbench } from "@/modules/filesystem/components/FilesystemWorkbench";
import { SharedWithMePanel } from "@/modules/filesystem/components/SharedWithMePanel";
import { resolveFileSpaceView } from "@/modules/filesystem/components/file-space-view";
import {
  loadAccessibleFileNamespaces,
  loadGovernedFileChildren,
  loadFileNode,
  loadSharedFileItems,
  resolveFileSpaceLoad,
  resolveFileNodeSelection,
} from "@/server/loaders/filesystem";
import { loadFilesystemConsoleContext } from "../../_lib/console-context";
import { normalizeFilesPageSearch } from "../../page-state";

export default async function FilesDataWorkspacePage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const ctx = await loadFilesystemConsoleContext();
  if (!ctx.ok) {
    return ctx.element;
  }
  const {
    session,
    workspaceId,
    organizationId,
    organizationLabel,
    workspaceLabel,
    permissions,
    members,
    memberDirectoryAvailable,
  } = ctx.value;

  const normalizedSearch = normalizeFilesPageSearch(await searchParams);
  if (normalizedSearch.redirectPath) {
    redirect(normalizedSearch.redirectPath);
  }
  const initialState = { ...normalizedSearch.state, tab: "files" as const };

  const namespaceLoad = await resolveFileSpaceLoad(() =>
    loadAccessibleFileNamespaces(organizationId, workspaceId),
  );
  if (namespaceLoad.kind !== "loaded") {
    const message =
      namespaceLoad.kind === "forbidden"
        ? "无权读取当前租户的文件空间。"
        : namespaceLoad.kind === "not-found"
          ? "当前租户的文件空间不存在。"
          : "文件空间加载失败，请稍后重试。";
    return (
      <div className="filesystem-data-workspace stack-sm min-w-0">
        <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} />
        <InlineAlert tone="error">{message}</InlineAlert>
      </div>
    );
  }

  const spaceView = resolveFileSpaceView({
    space: initialState.space,
    summaries: namespaceLoad.value.items,
    organizationId,
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
      <div className="filesystem-data-workspace stack-sm min-w-0">
        <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} />
        <InlineAlert tone="error">{message}</InlineAlert>
      </div>
    );
  }

  if (spaceView.kind === "shared") {
    const sharedLoad = await resolveFileSpaceLoad(() =>
      loadSharedFileItems(organizationId, workspaceId, {
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
      <div className="filesystem-data-workspace stack-sm min-w-0">
        <FileSpaceTabList activeSpace="shared" view={initialState.view} />
        <SharedWithMePanel
          workspaceId={workspaceId}
          organizationId={organizationId}
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
      <div className="filesystem-data-workspace stack-sm min-w-0">
        <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} />
        <FileSpaceNav
          activeSpace={initialState.space}
          organizationLabel={organizationLabel}
          workspaceLabel={workspaceLabel}
          namespace={namespaceSummary}
          workspaceId={workspaceId}
        />
      </div>
    );
  }

  const listingLoad = await resolveFileSpaceLoad(() =>
    loadGovernedFileChildren(organizationId, workspaceId, namespaceSummary.id, {
      parentId: initialState.parent ?? rootId,
      cursor: initialState.cursor,
      query: initialState.query,
      limit: 100,
    }),
  );
  if (listingLoad.kind !== "loaded") {
    const message =
      listingLoad.kind === "forbidden"
        ? "无权访问所选目录。"
        : listingLoad.kind === "not-found"
          ? "所选目录不存在或已被移除。"
          : "目录内容加载失败，请稍后重试。";
    return (
      <div className="filesystem-data-workspace stack-sm min-w-0">
        <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} />
        <div className="filesystem-workspace-frame filesystem-browser-layout">
          <aside aria-label="目录" className="filesystem-directory-sidebar">
            <FileSpaceNav
              activeSpace={initialState.space}
              organizationLabel={organizationLabel}
              workspaceLabel={workspaceLabel}
              namespace={namespaceSummary}
              workspaceId={workspaceId}
            />
          </aside>
        </div>
        <InlineAlert tone="error">{message}</InlineAlert>
      </div>
    );
  }

  const listing = listingLoad.value;
  const parentId = initialState.parent ?? rootId;
  let currentFolderLabel = "根目录";
  if (initialState.parent && initialState.parent !== rootId) {
    const parentSelection = await resolveFileNodeSelection(namespaceSummary.id, () =>
      loadFileNode(workspaceId, parentId, namespaceSummary.id),
    );
    if (parentSelection.node?.name) {
      currentFolderLabel = parentSelection.node.name;
    }
  }
  const selection = initialState.node
    ? await resolveFileNodeSelection(namespaceSummary.id, () =>
        loadFileNode(workspaceId, initialState.node!, namespaceSummary.id),
      )
    : {};
  const namespace = {
    id: namespaceSummary.id,
    workspace_id: workspaceId,
    root_node_id: namespaceSummary.root_node_id ?? null,
    status: namespaceSummary.status,
  };

  return (
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
      organizationId={organizationId}
      members={members}
      memberDirectoryAvailable={memberDirectoryAvailable}
      lockedTab="files"
      hideTabList
      currentFolderLabel={currentFolderLabel}
    />
  );
}