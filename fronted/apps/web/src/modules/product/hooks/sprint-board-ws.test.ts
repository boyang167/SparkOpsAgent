import { describe, expect, it } from "vitest";
import { buildSprintBoardWebSocketURL } from "./sprint-board-ws";

describe("buildSprintBoardWebSocketURL", () => {
  it("builds dev websocket url with auth query params", () => {
    const url = buildSprintBoardWebSocketURL({
      websocketPath: "/api/v1/products/p1/sprints/s1/board/ws",
      accessToken: "token-abc",
      workspaceId: "ws-1",
      location: {
        protocol: "http:",
        hostname: "localhost",
        host: "localhost:3000",
        origin: "http://localhost:3000",
      },
      isDev: true,
    });
    expect(url).toBe(
      "ws://localhost:8080/api/v1/products/p1/sprints/s1/board/ws?access_token=token-abc&workspace_id=ws-1",
    );
  });
});
