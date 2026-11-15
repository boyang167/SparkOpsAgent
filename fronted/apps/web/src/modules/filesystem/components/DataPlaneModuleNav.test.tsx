// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { createElement } from "react";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DataPlaneModuleNav } from "./DataPlaneModuleNav";

vi.mock("next/navigation", () => ({
  usePathname: () => "/files/data/workspace",
}));

describe("DataPlaneModuleNav", () => {
  afterEach(cleanup);

  it("links to workspace and transfers", () => {
    render(createElement(DataPlaneModuleNav));
    expect(screen.getByRole("link", { name: "文件浏览" })).toHaveAttribute(
      "href",
      "/files/data/workspace",
    );
    expect(screen.getByRole("link", { name: "传输中心" })).toHaveAttribute(
      "href",
      "/files/data/transfers",
    );
  });
});