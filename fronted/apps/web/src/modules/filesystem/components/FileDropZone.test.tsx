// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FileDropZone } from "./FileDropZone";
import { FILE_NODE_DRAG_TYPE } from "./directory-dnd";

function transfer(files: File[], types = ["Files"], directory = false) {
  return {
    files,
    types,
    items: files.map(() => ({
      webkitGetAsEntry: () => ({ isDirectory: directory }),
    })),
  };
}

function renderDropZone(onFiles: (files: File[], parentId: string) => void, canWrite = true) {
  return render(
    createElement(
      FileDropZone,
      {
        targetParentId: "target-parent",
        targetName: "交付中心",
        canWrite,
        onFiles,
      },
      createElement("div", null, "目录内容"),
    ),
  );
}

describe("FileDropZone", () => {
  afterEach(cleanup);

  it("accepts multiple desktop files for the current directory", () => {
    const onFiles = vi.fn();
    const files = [new File(["a"], "a.txt"), new File(["b"], "b.txt")];
    renderDropZone(onFiles);
    const zone = screen.getByTestId("file-drop-zone");

    fireEvent.dragEnter(zone, { dataTransfer: transfer(files) });
    expect(screen.getByText('释放以上传到“交付中心”')).toBeInTheDocument();
    fireEvent.drop(zone, { dataTransfer: transfer(files) });

    expect(onFiles).toHaveBeenCalledWith(files, "target-parent");
  });

  it("rejects local directories without starting an upload", () => {
    const onFiles = vi.fn();
    const files = [new File([""], "folder")];
    renderDropZone(onFiles);
    const zone = screen.getByTestId("file-drop-zone");

    fireEvent.drop(zone, { dataTransfer: transfer(files, ["Files"], true) });

    expect(onFiles).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("当前版本不支持目录递归上传");
  });

  it("never treats an internal node drag as a desktop upload", () => {
    const onFiles = vi.fn();
    const files = [new File(["a"], "a.txt")];
    renderDropZone(onFiles);
    const zone = screen.getByTestId("file-drop-zone");
    const dataTransfer = transfer(files, ["Files", FILE_NODE_DRAG_TYPE]);

    fireEvent.dragEnter(zone, { dataTransfer });
    fireEvent.drop(zone, { dataTransfer });

    expect(onFiles).not.toHaveBeenCalled();
    expect(screen.queryByText(/释放以上传/)).not.toBeInTheDocument();
  });

  it("does not accept files without write permission", () => {
    const onFiles = vi.fn();
    const files = [new File(["a"], "a.txt")];
    renderDropZone(onFiles, false);
    const zone = screen.getByTestId("file-drop-zone");

    fireEvent.dragEnter(zone, { dataTransfer: transfer(files) });
    fireEvent.drop(zone, { dataTransfer: transfer(files) });

    expect(onFiles).not.toHaveBeenCalled();
    expect(screen.queryByText(/释放以上传/)).not.toBeInTheDocument();
  });
});
