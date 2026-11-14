import {
  parseWorkbenchState,
  type WorkbenchState,
} from "@/modules/filesystem/components/workbench-state";

type SearchParamValue = string | string[] | undefined;

function firstValue(value: SearchParamValue) {
  return Array.isArray(value) ? value[0] : value;
}

function safeFilesPath(state: WorkbenchState) {
  const params = new URLSearchParams();
  if (state.tab !== "files") params.set("tab", state.tab);
  if (state.view !== "list") params.set("view", state.view);
  if (state.parent) params.set("parent", state.parent);
  if (state.node) params.set("node", state.node);
  const query = params.toString();
  return query ? `/files?${query}` : "/files";
}

export function normalizeFilesPageSearch(
  input: Record<string, SearchParamValue>,
): { state: WorkbenchState; redirectPath?: string } {
  const state = parseWorkbenchState(input);
  const query = firstValue(input.q)?.trim() ?? "";
  if (query.length <= 200) {
    return { state };
  }
  const safeState = { ...state, query: "", cursor: undefined };
  return {
    state: safeState,
    redirectPath: safeFilesPath(safeState),
  };
}
