// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement, useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { ShareApprovalPanel } from "./ShareApprovalPanel";
import { ShareFileModal } from "./ShareFileModal";

const sdk = vi.hoisted(() => ({
  approveShareRequest: vi.fn(),
  createGovernedGrant: vi.fn(),
  createShareRequest: vi.fn(),
  listOrganizationShareRequests: vi.fn(),
  previewGovernedShare: vi.fn(),
  rejectShareRequest: vi.fn(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => sdk);
vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ tone, children }: { tone: string; children: React.ReactNode }) =>
    createElement("div", { className: `inline-alert inline-alert-${tone}` }, children),
}));

const node: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: "00000000-0000-4000-8000-000000000012",
  parent_id: null,
  type: "DIRECTORY",
  name: "设计资料",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000013",
  classification: "INTERNAL",
};

const pending = {
  id: "00000000-0000-4000-8000-000000000021",
  organization_id: "00000000-0000-4000-8000-000000000001",
  namespace_id: node.namespace_id,
  node_id: node.id,
  requested_by: node.owner_id,
  principal_type: "ORGANIZATION",
  principal_ids: ["00000000-0000-4000-8000-000000000001"],
  actions: ["READ"],
  classification: "INTERNAL",
  effect: "ALLOW",
  status: "PENDING",
  created_at: "2026-11-15T00:00:00Z",
};

function ShareModalHarness() {
  const [open, setOpen] = useState(false);
  return createElement(
    "div",
    null,
    createElement("button", { type: "button", onClick: () => setOpen(true) }, "打开共享"),
    createElement(ShareFileModal, {
      open,
      node,
      organizationId: pending.organization_id,
      organizationLabel: "星火组织",
      workspaceId: "00000000-0000-4000-8000-000000000002",
      workspaceLabel: "研发空间",
      members: [],
      effectiveActions: ["SHARE"],
      onClose: () => setOpen(false),
    }),
  );
}

describe("enterprise governance uses the production Modal accessibility contract", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sdk.previewGovernedShare.mockResolvedValue({ outcome: "ACTIVE", reason: "POLICY_ACTIVE" });
    sdk.listOrganizationShareRequests.mockResolvedValue({
      items: [pending],
      next_cursor: "",
      truncated: false,
      total_count: 1,
    });
  });
  afterEach(cleanup);

  it("ShareFileModal exposes ARIA, traps focus, closes with Escape and restores its trigger", async () => {
    const user = userEvent.setup();
    render(createElement(ShareModalHarness));
    const trigger = screen.getByRole("button", { name: "打开共享" });
    await user.click(trigger);

    const dialog = await screen.findByRole("dialog", { name: "共享“设计资料”" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    const titleID = dialog.getAttribute("aria-labelledby");
    const descriptionID = dialog.getAttribute("aria-describedby");
    expect(document.getElementById(titleID!)).toHaveTextContent("共享“设计资料”");
    expect(document.getElementById(descriptionID!)).toHaveTextContent("选择可验证的受众");

    const close = screen.getByRole("button", { name: "关闭" });
    await waitFor(() => expect(close).toHaveFocus());
    const cancel = screen.getByRole("button", { name: "取消" });
    fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
    expect(cancel).toHaveFocus();
    fireEvent.keyDown(cancel, { key: "Tab" });
    expect(close).toHaveFocus();

    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  it("approval rejection uses production Modal ARIA, focus trap and trigger restoration", async () => {
    const user = userEvent.setup();
    render(createElement(ShareApprovalPanel, {
      organizationId: pending.organization_id,
      workspaceId: "00000000-0000-4000-8000-000000000002",
    }));
    const trigger = await screen.findByRole("button", { name: "拒绝共享申请" });
    await user.click(trigger);

    const dialog = await screen.findByRole("dialog", { name: "拒绝共享申请" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(document.getElementById(dialog.getAttribute("aria-labelledby")!)).toHaveTextContent("拒绝共享申请");
    expect(document.getElementById(dialog.getAttribute("aria-describedby")!)).toHaveTextContent(
      "请填写可供申请人理解的拒绝原因。",
    );

    const close = screen.getByRole("button", { name: "关闭" });
    await waitFor(() => expect(close).toHaveFocus());
    const cancel = screen.getByRole("button", { name: "取消" });
    fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
    expect(cancel).toHaveFocus();
    fireEvent.keyDown(cancel, { key: "Tab" });
    expect(close).toHaveFocus();

    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });
});
