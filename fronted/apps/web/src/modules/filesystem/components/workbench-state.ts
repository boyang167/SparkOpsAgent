export type WorkbenchTab = "files" | "transfers" | "governance" | "audit";
export type FileView = "list" | "grid";

export type WorkbenchState = {
  tab: WorkbenchTab;
  view: FileView;
  parent?: string;
  cursor?: string;
  query: string;
};

export function parseWorkbenchState(input: Record<string, string | undefined>): WorkbenchState {
  const tabs: WorkbenchTab[] = ["files", "transfers", "governance", "audit"];
  return {
    tab: tabs.includes(input.tab as WorkbenchTab) ? (input.tab as WorkbenchTab) : "files",
    view: input.view === "grid" ? "grid" : "list",
    parent: input.parent || undefined,
    cursor: input.cursor || undefined,
    query: input.q?.trim() ?? "",
  };
}
