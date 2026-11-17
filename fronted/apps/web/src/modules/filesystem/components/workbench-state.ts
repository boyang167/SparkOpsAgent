export type WorkbenchTab = "files" | "transfers" | "governance" | "approvals" | "audit";
export type FileView = "list" | "grid";
export type FileSpace = "my" | "workspace" | "organization" | "shared";
export type SharedSource = "organization" | "workspace" | "user" | "agent";

export type WorkbenchState = {
  tab: WorkbenchTab;
  view: FileView;
  space?: FileSpace;
  source?: SharedSource;
  parent?: string;
  cursor?: string;
  query: string;
  node?: string;
};
export type ParsedWorkbenchState = WorkbenchState & { space: FileSpace };

type SearchParamValue = string | string[] | undefined;

function firstValue(value: SearchParamValue): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

export function hrefForDataWorkspace(space: FileSpace = "workspace", view: FileView = "list") {
  const params = new URLSearchParams();
  if (view !== "list") params.set("view", view);
  if (space !== "workspace") params.set("space", space);
  const query = params.toString();
  return query ? `/files/data/workspace?${query}` : "/files/data/workspace";
}

export function hrefForDataTrash(space: FileSpace = "workspace", view: FileView = "list") {
  const params = new URLSearchParams();
  if (view !== "list") params.set("view", view);
  if (space !== "workspace") params.set("space", space);
  const query = params.toString();
  return query ? `/files/data/trash?${query}` : "/files/data/trash";
}

export function parseWorkbenchState(input: Record<string, SearchParamValue>): ParsedWorkbenchState {
  const tabs: WorkbenchTab[] = ["files", "transfers", "governance", "approvals", "audit"];
  const tab = firstValue(input.tab);
  const view = firstValue(input.view);
  const parent = firstValue(input.parent);
  const cursor = firstValue(input.cursor);
  const query = firstValue(input.q);
  const node = firstValue(input.node);
  const requestedSpace = firstValue(input.space);
  const spaces: FileSpace[] = ["my", "workspace", "organization", "shared"];
  const space = spaces.includes(requestedSpace as FileSpace)
    ? (requestedSpace as FileSpace)
    : "workspace";
  const requestedSource = firstValue(input.source);
  const sources: SharedSource[] = ["organization", "workspace", "user", "agent"];
  const source =
    space === "shared" && sources.includes(requestedSource as SharedSource)
      ? (requestedSource as SharedSource)
      : undefined;

  return {
    tab: tabs.includes(tab as WorkbenchTab) ? (tab as WorkbenchTab) : "files",
    view: view === "grid" ? "grid" : "list",
    space,
    source,
    parent: space === "shared" ? undefined : parent || undefined,
    cursor: cursor || undefined,
    query: query?.trim() ?? "",
    node: node || undefined,
  };
}
