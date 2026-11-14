// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ShareApprovalPanel } from "./ShareApprovalPanel";

const sdk = vi.hoisted(() => ({
  listOrganizationShareRequests: vi.fn(),
  approveShareRequest: vi.fn(),
  rejectShareRequest: vi.fn(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => sdk);
vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ children }: { children: React.ReactNode }) => createElement("div", null, children),
}));
vi.mock("@/components/ui/Modal", () => ({
  Modal: ({ open, title, children, footer }: { open: boolean; title: string; children: React.ReactNode; footer?: React.ReactNode }) =>
    open ? createElement("div", { role: "dialog", "aria-label": title }, children, footer) : null,
}));

const pending = {
  id: "00000000-0000-4000-8000-000000000011",
  organization_id: "00000000-0000-4000-8000-000000000001",
  namespace_id: "00000000-0000-4000-8000-000000000012",
  node_id: "00000000-0000-4000-8000-000000000013",
  requested_by: "00000000-0000-4000-8000-000000000014",
  principal_type: "ORGANIZATION",
  principal_ids: ["00000000-0000-4000-8000-000000000001"],
  actions: ["READ"],
  classification: "INTERNAL",
  effect: "ALLOW",
  status: "PENDING",
  created_at: "2026-11-14T04:00:00Z",
};

describe("ShareApprovalPanel", () => {
  beforeEach(() => {
    sdk.listOrganizationShareRequests.mockReset();
    sdk.approveShareRequest.mockReset();
    sdk.rejectShareRequest.mockReset();
    sdk.listOrganizationShareRequests.mockResolvedValue({
      items: [pending],
      next_cursor: "",
      truncated: false,
      total_count: 1,
    });
    sdk.approveShareRequest.mockResolvedValue({ ...pending, status: "APPROVED" });
    sdk.rejectShareRequest.mockResolvedValue({ ...pending, status: "REJECTED" });
  });

  afterEach(() => cleanup());

  it("loads pending requests and refreshes count and list after approval", async () => {
    const user = userEvent.setup();
    render(createElement(ShareApprovalPanel, {
      organizationId: pending.organization_id,
      workspaceId: "00000000-0000-4000-8000-000000000002",
    }));

    expect(await screen.findByText("待审批 1")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "批准共享申请" }));
    await waitFor(() => expect(sdk.approveShareRequest).toHaveBeenCalledTimes(1));
    expect(sdk.approveShareRequest.mock.calls[0]?.[1]?.idempotencyKey).toMatch(/^share-approve-/);
    expect(sdk.listOrganizationShareRequests).toHaveBeenCalledTimes(2);
  });

  it("requires a bounded rejection reason and prevents duplicate submission", async () => {
    const user = userEvent.setup();
    render(createElement(ShareApprovalPanel, {
      organizationId: pending.organization_id,
      workspaceId: "00000000-0000-4000-8000-000000000002",
    }));
    await screen.findByText("待审批 1");
    await user.click(screen.getByRole("button", { name: "拒绝共享申请" }));
    const submit = screen.getByRole("button", { name: "确认拒绝" });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText("拒绝原因"), "受众范围不符合组织策略");
    await user.click(submit);
    await waitFor(() => expect(sdk.rejectShareRequest).toHaveBeenCalledTimes(1));
    expect(sdk.rejectShareRequest).toHaveBeenCalledWith(
      pending.id,
      { reason: "受众范围不符合组织策略" },
      expect.objectContaining({ idempotencyKey: expect.stringMatching(/^share-reject-/) }),
    );
  });

  it("shows the authoritative pending total and loads the next page without replacing the first page", async () => {
    const second = {
      ...pending,
      id: "00000000-0000-4000-8000-000000000099",
      node_id: "00000000-0000-4000-8000-000000000098",
    };
    sdk.listOrganizationShareRequests
      .mockResolvedValueOnce({
        items: [pending],
        next_cursor: "page-2",
        truncated: true,
        total_count: 51,
      })
      .mockResolvedValueOnce({
        items: [second],
        next_cursor: "",
        truncated: false,
        total_count: 51,
      });
    const user = userEvent.setup();
    render(createElement(ShareApprovalPanel, {
      organizationId: pending.organization_id,
      workspaceId: "00000000-0000-4000-8000-000000000002",
    }));

    expect(await screen.findByText("待审批 51")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "加载更多待审批申请" }));
    await waitFor(() => expect(sdk.listOrganizationShareRequests).toHaveBeenCalledTimes(2));
    expect(sdk.listOrganizationShareRequests.mock.calls[1]?.[0]).toEqual({
      status: "PENDING",
      limit: 50,
      cursor: "page-2",
    });
    expect(screen.getAllByRole("button", { name: "批准共享申请" })).toHaveLength(2);
  });
});
