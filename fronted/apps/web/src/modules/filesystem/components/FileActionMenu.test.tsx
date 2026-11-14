// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { FileActionMenu } from "./FileActionMenu";

const sdk = vi.hoisted(() => ({ getEffectiveAccess: vi.fn() }));
vi.mock("@sparkops/web-sdk/filesystem", () => sdk);

const node: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: "00000000-0000-4000-8000-000000000002",
  parent_id: null,
  type: "FILE",
  name: "报告.pdf",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000013",
  classification: "CONFIDENTIAL",
};

function menu(onShare = vi.fn()) {
  return createElement(FileActionMenu, {
    node,
    canRename: false,
    canMove: false,
    organizationId: "00000000-0000-4000-8000-000000000021",
    workspaceId: "00000000-0000-4000-8000-000000000022",
    onShare,
    onRename: vi.fn(),
    onMove: vi.fn(),
  });
}

describe("FileActionMenu share authorization", () => {
  beforeEach(() => sdk.getEffectiveAccess.mockReset());
  afterEach(cleanup);

  it("does not render share without effective SHARE", async () => {
    sdk.getEffectiveAccess.mockResolvedValue({ effective_actions: ["READ"] });
    render(menu());
    expect(await screen.findByLabelText("报告.pdf 操作")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "共享 报告.pdf" })).not.toBeInTheDocument();
  });

  it("renders and invokes share only with effective SHARE", async () => {
    const onShare = vi.fn();
    sdk.getEffectiveAccess.mockResolvedValue({ effective_actions: ["READ", "SHARE"] });
    render(menu(onShare));
    await userEvent.click(await screen.findByRole("button", { name: "共享 报告.pdf" }));
    expect(onShare).toHaveBeenCalledWith(node, ["READ", "SHARE"]);
  });
});
