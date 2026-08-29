import { describe, expect, it } from "vitest";
import {
  isCurrentWorkspaceRequest,
  selectWorkspaceAuthorization,
} from "./cluster-authorization-state";

describe("workspace cluster authorization state", () => {
  it("clears stale authorization state when workspace changes", () => {
    expect(
      selectWorkspaceAuthorization(
        { workspaceId: "old", generation: 3, items: [{ id: "c" }], error: "old" },
        "new",
      ),
    ).toEqual({
      workspaceId: "new",
      generation: 4,
      items: [],
      error: null,
      loading: true,
    });
  });

  it("rejects a response from a previous workspace selection", () => {
    expect(
      isCurrentWorkspaceRequest(
        { workspaceId: "new", generation: 4 },
        { workspaceId: "old", generation: 3 },
      ),
    ).toBe(false);
  });
});
