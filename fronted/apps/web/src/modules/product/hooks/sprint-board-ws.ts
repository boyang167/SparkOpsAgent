type BrowserLocation = {
  protocol: string;
  hostname: string;
  host: string;
  origin: string;
};

export function buildSprintBoardWebSocketURL(options: {
  websocketPath: string;
  accessToken: string;
  workspaceId: string;
  configuredBase?: string;
  location: BrowserLocation;
  isDev: boolean;
}) {
  const configured = options.configuredBase?.trim().replace(/\/+$/, "");
  let base: string;
  if (configured) {
    base = configured.replace(/^http:/, "ws:").replace(/^https:/, "wss:");
  } else if (options.isDev) {
    base = `ws://${options.location.hostname}:8080`;
  } else {
    base = options.location.origin.replace(/^http:/, "ws:").replace(/^https:/, "wss:");
  }
  const path = options.websocketPath.startsWith("/") ? options.websocketPath : `/${options.websocketPath}`;
  const params = new URLSearchParams({
    access_token: options.accessToken,
    workspace_id: options.workspaceId,
  });
  return `${base}${path}?${params.toString()}`;
}
