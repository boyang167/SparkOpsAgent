export type WorkbenchTab = "files" | "transfers" | "governance" | "audit";
export type FileView = "list" | "grid";

export type WorkbenchState = {
  tab: WorkbenchTab;
  view: FileView;
  parent?: string;
  cursor?: string;
  query: string;
  node?: string;
};

type SearchParamValue = string | string[] | undefined;

function firstValue(value: SearchParamValue): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

export function parseWorkbenchState(input: Record<string, SearchParamValue>): WorkbenchState {
  const tabs: WorkbenchTab[] = ["files", "transfers", "governance", "audit"];
  const tab = firstValue(input.tab);
  const view = firstValue(input.view);
  const parent = firstValue(input.parent);
  const cursor = firstValue(input.cursor);
  const query = firstValue(input.q);
  const node = firstValue(input.node);

  return {
    tab: tabs.includes(tab as WorkbenchTab) ? (tab as WorkbenchTab) : "files",
    view: view === "grid" ? "grid" : "list",
    parent: parent || undefined,
    cursor: cursor || undefined,
    query: query?.trim() ?? "",
    node: node || undefined,
  };
}
