// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GovernanceWorkspace } from "./GovernanceWorkspace";

vi.mock("./StoragePoolPanel", () => ({
  StoragePoolPanel: () => createElement("div", null, "Provider 与存储池内容"),
}));
vi.mock("./StorageBindingPanel", () => ({
  StorageBindingPanel: () => createElement("div", null, "作用域绑定内容"),
}));
vi.mock("./LifecyclePolicyPanel", () => ({
  LifecyclePolicyPanel: () => createElement("div", null, "生命周期策略内容"),
}));

describe("GovernanceWorkspace", () => {
  afterEach(() => cleanup());

  it("renders only the selected governance panel", async () => {
    const user = userEvent.setup();
    render(
      createElement(GovernanceWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: ["storage:read", "storage:admin", "lifecycle:admin"],
      }),
    );

    expect(screen.getByText("Provider 与存储池内容")).toBeInTheDocument();
    expect(screen.queryByText("作用域绑定内容")).not.toBeInTheDocument();
    expect(screen.queryByText("生命周期策略内容")).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "作用域绑定" }));
    expect(screen.getByText("作用域绑定内容")).toBeInTheDocument();
    expect(screen.queryByText("Provider 与存储池内容")).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "生命周期策略" }));
    expect(screen.getByText("生命周期策略内容")).toBeInTheDocument();
    expect(screen.queryByText("作用域绑定内容")).not.toBeInTheDocument();
  });

  it("shows a distinct forbidden state without storage read permission", () => {
    render(
      createElement(GovernanceWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: [],
      }),
    );

    expect(screen.getByText("无权查看存储治理。")).toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  });
});
