// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { ShareFileModal } from "./ShareFileModal";

const sdk = vi.hoisted(() => ({
  createGovernedGrant: vi.fn(),
  createShareRequest: vi.fn(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => sdk);
vi.mock("@/components/ui/Modal", () => ({
  Modal: ({ open, title, children, footer }: {
    open: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) => open ? createElement("div", { role: "dialog", "aria-label": title }, children, footer) : null,
}));
vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ children }: { children: React.ReactNode }) =>
    createElement("div", { role: "alert" }, children),
}));

const node: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: "00000000-0000-4000-8000-000000000002",
  parent_id: "00000000-0000-4000-8000-000000000003",
  type: "DIRECTORY",
  name: "设计资料",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000013",
  classification: "INTERNAL",
};

function modal(overrides: Partial<React.ComponentProps<typeof ShareFileModal>> = {}) {
  return createElement(ShareFileModal, {
    open: true,
    node,
    organizationId: "00000000-0000-4000-8000-000000000101",
    organizationLabel: "星火组织",
    workspaceId: "00000000-0000-4000-8000-000000000102",
    workspaceLabel: "研发空间",
    members: [
      {
        member_id: "00000000-0000-4000-8000-000000000103",
        email: "member@example.com",
        display_name: "王小明",
        roles: [],
      },
    ],
    effectiveActions: ["SHARE"],
    onClose: vi.fn(),
    ...overrides,
  });
}

describe("ShareFileModal", () => {
  beforeEach(() => {
    sdk.createGovernedGrant.mockReset();
    sdk.createShareRequest.mockReset();
  });
  afterEach(cleanup);

  it("uses authoritative audience choices and never accepts arbitrary identifiers", () => {
    render(modal());
    expect(screen.getByRole("radio", { name: /研发空间/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /王小明/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /星火组织/ })).toBeDisabled();
    expect(screen.queryByRole("textbox", { name: /ID|UUID/ })).not.toBeInTheDocument();
    expect(screen.getByText(/成员目录不可用/)).toBeInTheDocument();
  });

  it("blocks organization audience for restricted nodes", () => {
    render(modal({ node: { ...node, classification: "RESTRICTED" } }));
    expect(screen.getByRole("radio", { name: /星火组织/ })).toBeDisabled();
    expect(screen.getByText(/受限内容不能共享给整个组织/)).toBeInTheDocument();
  });

  it("submits matching workspace data with one stable idempotency key", async () => {
    const user = userEvent.setup();
    sdk.createShareRequest.mockResolvedValue({
      id: "00000000-0000-4000-8000-000000000201",
      status: "PENDING",
    });
    render(modal());

    await user.click(screen.getByRole("radio", { name: /研发空间/ }));
    expect(screen.getByText("将影响研发空间的 1 位成员")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "下一步" }));
    expect(screen.getByRole("checkbox", { name: "READ" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "DOWNLOAD" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /应用到子项/ })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "提交审批" }));

    await waitFor(() => expect(sdk.createShareRequest).toHaveBeenCalledTimes(1));
    expect(sdk.createShareRequest).toHaveBeenCalledWith(
      node.id,
      expect.objectContaining({
        principal_type: "WORKSPACE",
        principal_ids: ["00000000-0000-4000-8000-000000000102"],
        actions: ["READ", "DOWNLOAD"],
        effect: "ALLOW",
        inherit_to_children: true,
      }),
      expect.objectContaining({
        organizationId: "00000000-0000-4000-8000-000000000101",
        workspaceId: "00000000-0000-4000-8000-000000000102",
        idempotencyKey: expect.any(String),
      }),
    );
    expect(await screen.findByText("等待审批")).toBeInTheDocument();
  });

  it("defensively refuses submission without effective SHARE", async () => {
    const user = userEvent.setup();
    render(modal({ effectiveActions: ["READ"] }));
    await user.click(screen.getByRole("radio", { name: /王小明/ }));
    await user.click(screen.getByRole("button", { name: "下一步" }));
    await user.click(screen.getByRole("button", { name: "立即共享" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("无权共享");
    expect(sdk.createGovernedGrant).not.toHaveBeenCalled();
  });
});
