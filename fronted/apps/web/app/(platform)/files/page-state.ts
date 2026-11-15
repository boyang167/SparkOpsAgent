import {
  parseWorkbenchState,
  type FileSpace,
  type ParsedWorkbenchState,
  type SharedSource,
  type WorkbenchState,
} from "@/modules/filesystem/components/workbench-state";

type SearchParamValue = string | string[] | undefined;

function firstValue(value: SearchParamValue) {
  return Array.isArray(value) ? value[0] : value;
}

function safeFilesPath(state: WorkbenchState) {
  const params = new URLSearchParams();
  if (state.view !== "list") params.set("view", state.view);
  if (state.space && state.space !== "workspace") params.set("space", state.space);
  if (state.space === "shared" && state.source) params.set("source", state.source);
  if (state.parent) params.set("parent", state.parent);
  if (state.node) params.set("node", state.node);
  if (state.query) params.set("q", state.query);
  const query = params.toString();
  const base = "/files/data/workspace";
  return query ? `${base}?${query}` : base;
}

export function fileSpacePath(
  space: FileSpace,
  state: WorkbenchState,
  requestedSource?: string,
) {
  const next: WorkbenchState = {
    ...state,
    space,
    source:
      space === "shared" &&
      ["organization", "workspace", "user", "agent"].includes(requestedSource ?? "")
        ? (requestedSource as SharedSource)
        : undefined,
    parent: undefined,
    cursor: undefined,
    node: undefined,
  };
  return safeFilesPath(next);
}

export function normalizeFilesPageSearch(
  input: Record<string, SearchParamValue>,
): { state: ParsedWorkbenchState; redirectPath?: string } {
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
