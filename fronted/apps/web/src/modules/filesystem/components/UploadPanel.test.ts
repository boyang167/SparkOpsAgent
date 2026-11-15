// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { UploadPanel } from "./UploadPanel";

describe("UploadPanel", () => {
  afterEach(cleanup);

  it("passes all selected files to the workbench queue without requiring a provider", () => {
    const onFilesSelected = vi.fn();
    const { container } = render(
      createElement(UploadPanel, { onFilesSelected }),
    );
    const input = container.querySelector("input[type=file]") as HTMLInputElement;
    const files = [new File(["a"], "a.txt"), new File(["b"], "b.txt")];

    expect(screen.getByLabelText("上传文件")).toBeEnabled();
    fireEvent.change(input, { target: { files } });

    expect(onFilesSelected).toHaveBeenCalledWith(files);
  });

  it("stays disabled when write permission is absent", () => {
    render(
      createElement(UploadPanel, {
        onFilesSelected: vi.fn(),
        disabled: true,
      }),
    );

    expect(screen.getByRole("button", { name: "上传文件" })).toBeDisabled();
  });
});
