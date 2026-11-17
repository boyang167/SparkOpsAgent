import { InlineAlert } from "@/components/ui/InlineAlert";
import { FileSpaceTabList } from "@/modules/filesystem/components/FileSpaceTabList";
import { TrashExplorerPanel } from "@/modules/filesystem/components/TrashExplorerPanel";
import { resolveFileSpaceView } from "@/modules/filesystem/components/file-space-view";
import { parseWorkbenchState } from "@/modules/filesystem/components/workbench-state";
import {
  loadAccessibleFileNamespaces,
  loadTrashItems,
  resolveFileSpaceLoad,
} from "@/server/loaders/filesystem";
import { loadFilesystemConsoleContext } from "../../_lib/console-context";

function normalizeTrashSearch(
  input: Record<string, string | string[] | undefined>,
) {
  const state = parseWorkbenchState(input);
  return { state: { ...state, tab: "files" as const } };
}

export default async function FilesDataTrashPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const ctx = await loadFilesystemConsoleContext();
  if (!ctx.ok) {
    return ctx.element;
  }
  const { session, workspaceId, organizationId } = ctx.value;
  const { state: initialState } = normalizeTrashSearch(await searchParams);

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
        <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} mode="trash" />
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
  if (spaceView.kind === "shared") {
    return (
      <div className="filesystem-data-workspace stack-sm min-w-0">
        <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} mode="trash" />
        <InlineAlert tone="error">回收站不支持「共享」空间，请切换到个人、工作区或租户空间。</InlineAlert>
      </div>
    );
  }
  if (spaceView.kind === "workspace-unavailable" || spaceView.kind === "missing") {
    const message =
      spaceView.kind === "workspace-unavailable"
        ? "当前工作区不可用，无法解析文件空间。"
        : `${{ my: "个人", workspace: "工作区", organization: "租户" }[spaceView.space]}文件空间尚未创建或未向你授权。`;
    return (
      <div className="filesystem-data-workspace stack-sm min-w-0">
        <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} mode="trash" />
        <InlineAlert tone="error">{message}</InlineAlert>
      </div>
    );
  }

  const trashLoad = await resolveFileSpaceLoad(() =>
    loadTrashItems(workspaceId, {
      namespaceId: spaceView.namespace.id,
      cursor: initialState.cursor,
      limit: 100,
    }),
  );

  return (
    <div className="filesystem-data-workspace stack-sm min-w-0">
      <FileSpaceTabList activeSpace={initialState.space} view={initialState.view} mode="trash" />
      <TrashExplorerPanel
        workspaceId={workspaceId}
        namespaceId={spaceView.namespace.id}
        initialItems={trashLoad.kind === "loaded" ? trashLoad.value.items : []}
        initialNextCursor={trashLoad.kind === "loaded" ? trashLoad.value.next_cursor : ""}
        initialLoadError={trashLoad.kind === "error"}
        view={initialState.view}
      />
    </div>
  );
}