// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { FileSpaceTabList } from "./FileSpaceTabList";

describe("FileSpaceTabList", () => {
  afterEach(cleanup);

  it("renders four file space tabs with control-plane button styles", () => {
    render(createElement(FileSpaceTabList, { activeSpace: "workspace", view: "grid" }));
    expect(screen.getByRole("tablist", { name: "文件空间" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "我的文件" })).toHaveClass("btn", "btn-ghost");
    expect(screen.getByRole("tab", { name: "工作区文件" })).toHaveClass("btn", "btn-secondary");
    expect(screen.getByRole("tab", { name: "工作区文件" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "租户文件" })).toHaveAttribute(
      "href",
      "/files/data/workspace?view=grid&space=organization",
    );
  });

  it("marks shared tab active", () => {
    render(createElement(FileSpaceTabList, { activeSpace: "shared", view: "list" }));
    expect(screen.getByRole("tab", { name: "与我共享" })).toHaveClass("btn-secondary");
    expect(screen.getByRole("tab", { name: "与我共享" })).toHaveAttribute("aria-current", "page");
  });

  it("renders trash spaces without shared and keeps trash route", () => {
    render(createElement(FileSpaceTabList, { activeSpace: "my", view: "list", mode: "trash" }));
    expect(screen.getByRole("tablist", { name: "回收站所属空间" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "与我共享" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "个人空间" })).toHaveAttribute("href", "/files/data/trash?space=my");
    expect(screen.getByRole("tab", { name: "工作区空间" })).toHaveAttribute("href", "/files/data/trash");
  });
});