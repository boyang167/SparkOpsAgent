export type {
  TerminalClientMessage,
  TerminalServerMessage,
  TerminalTicketResponse,
} from "@/modules/plugins/kubernetes/pod-terminal";

export {
  buildTerminalWebSocketURL,
  clampTerminalSize,
  encodeClientMessage,
  focusTerminalWithoutScroll,
  parseServerMessage,
  splitTerminalInput,
} from "@/modules/plugins/kubernetes/pod-terminal";

export function projectPodTerminalTicketPath(
  workspaceId: string,
  projectId: string,
  podId: string,
) {
  return `/api/bff/v1/workspaces/${workspaceId}/projects/${projectId}/pods/${podId}/terminal-ticket`;
}
